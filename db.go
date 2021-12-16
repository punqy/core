package core

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"
	logger "github.com/sirupsen/logrus"
	"github.com/slmder/qbuilder"
	"go.uber.org/multierr"
	"reflect"
	"time"
)

type DatabaseConfig struct {
}

func NewConnection(driverName, dsn string) *sqlx.DB {
	return sqlx.MustConnect(driverName, dsn)
}

type txKeyType string

const txKey = txKeyType("db.tx")

const attrTransactional = "transactional"

type (
	Args       []interface{}
	StringMap  map[string]string
	Pagination struct {
		Limit  uint32
		Offset uint32
	}
)

type Dal interface {
	Connection() *sqlx.DB
	Transaction(ctx context.Context) *sqlx.Tx
	DoInsert(ctx context.Context, sql string, entity interface{}) (sql.Result, error)
	DoUpdate(ctx context.Context, sql string, entity interface{}) (sql.Result, error)
	DoSelect(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	DoSelectOne(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	Transactional(ctx context.Context, cb func(ctx context.Context) error) error
	SubSelect(sel string) *qbuilder.SelectBuilder
	BuildSelect(sel ...string) *qbuilder.SelectBuilder
	BuildInsert(into string) *qbuilder.InsertBuilder
	BuildUpdate(rel string) *qbuilder.UpdateBuilder
	BuildDelete(rel string) *qbuilder.DeleteBuilder
	ToArgsAndExpressions(conditions map[string]interface{}) ([]interface{}, []string)
	PipeErr(err error) error
	FindBy(ctx context.Context, tableName string, dest interface{}, cond qbuilder.Conditions, pag Pagination) error
	FindOneBy(ctx context.Context, tableName string, dest interface{}, cond qbuilder.Conditions) error
	SoftDelete(ctx context.Context, tableName string, id uuid.UUID) error
}

type dal struct {
	conn            *sqlx.DB
	transactions    Transactions
	profilerEnabled bool
}

func NewDAL(conn *sqlx.DB, tm Transactions) Dal {
	return &dal{conn: conn, transactions: tm, profilerEnabled: true}
}

func (d *dal) Connection() *sqlx.DB {
	return d.conn
}

func (d *dal) pipeQueryLog(ctx context.Context, query string, args []interface{}, call func() error) error {
	if !d.profilerEnabled {
		return call()
	}
	appContext, ok := ctx.Value(profileContextKey).(Profile)
	if !ok {
		return call()
	}
	start := time.Now()
	defer func(ctx context.Context) {
		appContext.AddQueryProfile(query, time.Now().Sub(start).Seconds(), args)
	}(ctx)
	return call()
}

func (d *dal) pipeResultQueryLog(ctx context.Context, query string, args []interface{}, call func() (sql.Result, error)) (sql.Result, error) {
	if !d.profilerEnabled {
		return call()
	}
	appContext, ok := ctx.Value(profileContextKey).(Profile)
	if !ok {
		return call()
	}
	start := time.Now()
	defer func(ctx context.Context) {
		appContext.AddQueryProfile(query, time.Now().Sub(start).Seconds(), args)
	}(ctx)
	return call()
}

func (d *dal) Transaction(ctx context.Context) *sqlx.Tx {
	tx := getTransactionFromContext(ctx)
	if tx == nil {
		logger.Error("transaction not found in given context")
	}
	return tx
}

func (d *dal) DoInsert(ctx context.Context, query string, entity interface{}) (sql.Result, error) {
	return d.pipeResultQueryLog(ctx, query, []interface{}{entity}, func() (sql.Result, error) {
		tx := getTransactionFromContext(ctx)
		if tx == nil {
			return d.Connection().NamedExecContext(ctx, query, entity)
		}
		return tx.NamedExecContext(ctx, query, entity)
	})
}

func (d *dal) DoUpdate(ctx context.Context, query string, entity interface{}) (sql.Result, error) {
	return d.pipeResultQueryLog(ctx, query, []interface{}{entity}, func() (sql.Result, error) {
		tx := getTransactionFromContext(ctx)
		if tx == nil {
			return d.Connection().NamedExecContext(ctx, query, entity)
		}
		return tx.NamedExecContext(ctx, query, entity)
	})
}

func (d *dal) DoSelectOne(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return d.pipeQueryLog(ctx, query, args, func() error {
		tx := getTransactionFromContext(ctx)
		if tx == nil {
			return d.Connection().GetContext(ctx, dest, query, args...)
		}
		return tx.Get(dest, query, args...)
	})
}

func (d *dal) DoSelect(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return d.pipeQueryLog(ctx, query, args, func() error {
		tx := getTransactionFromContext(ctx)
		if tx == nil {
			return d.Connection().SelectContext(ctx, dest, query, args...)
		}
		return tx.Select(dest, query, args...)
	})
}

func (d *dal) Transactional(ctx context.Context, cb func(ctx context.Context) error) error {
	return d.transactions.Run(ctx, cb)
}

func (d *dal) BuildSelect(sel ...string) *qbuilder.SelectBuilder {
	return qbuilder.Select(sel...)
}
func (d *dal) BuildSelectE(obj interface{}) *qbuilder.SelectBuilder {
	return qbuilder.SelectE(obj)
}

func (d *dal) SubSelect(sel string) *qbuilder.SelectBuilder {
	return qbuilder.SubSelect(sel)
}

func (d *dal) BuildInsert(into string) *qbuilder.InsertBuilder {
	return qbuilder.Insert(into)
}

func (d *dal) BuildUpdate(rel string) *qbuilder.UpdateBuilder {
	return qbuilder.Update(rel)
}

func (d *dal) BuildDelete(rel string) *qbuilder.DeleteBuilder {
	return qbuilder.Delete(rel)
}

func (d *dal) FindBy(ctx context.Context, tableName string, dest interface{}, cond qbuilder.Conditions, pager Pagination) error {
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Ptr {
		return Wrap(fmt.Errorf("must pass a pointer to slice of stuct, not a value, to FindBy destination %T", dest))
	}
	if value.IsNil() {
		return Wrap(errors.New("nil pointer passed to StructScan destination"))
	}
	e := value.Elem()
	if e.Kind() != reflect.Slice {
		return Wrap(fmt.Errorf("must pass a pointer to slice of stuct, not a value, to FindBy destination %T", e))
	}
	slice := reflectx.Deref(value.Type())
	base := reflectx.Deref(slice.Elem())
	if base.Kind() != reflect.Struct {
		return Wrap(fmt.Errorf("must pass a pointer to slice of stuct, not a value, to FindBy destination %T", e))
	}
	args, expressions := d.ToArgsAndExpressions(cond)
	query := d.BuildSelect().
		From(tableName).
		Where(expressions...).
		Limit(pager.Limit).
		Offset(pager.Offset).
		ToSQL()

	return d.PipeErr(d.DoSelect(ctx, dest, query, args...))
}

func (d *dal) FindOneBy(ctx context.Context, tableName string, dest interface{}, cond qbuilder.Conditions) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr {
		return Wrap(fmt.Errorf("must pass a pointer to a stuct, %T", dest))
	}
	e := v.Elem()
	if e.Kind() != reflect.Struct {
		return Wrap(fmt.Errorf("must pass a pointer to a stuct, %T", dest))
	}
	args, expressions := d.ToArgsAndExpressions(cond)
	query := d.BuildSelectE(dest).
		From(tableName).
		Where(expressions...).
		Limit(1).
		ToSQL()

	return d.PipeErr(d.DoSelectOne(ctx, dest, query, args...))
}

func (d *dal) SoftDelete(ctx context.Context, tableName string, id uuid.UUID) error {
	query := d.BuildUpdate(tableName).
		Set("deleted_at", "now()").
		Where("id = $1")
	_, err := d.Transaction(ctx).Exec(query.ToSQL(), id)
	return d.PipeErr(err)
}

type TransactionManagerConfig struct {
	IsolationLevel sql.IsolationLevel
}

func NewDefaultTransactionManagerConfig() TransactionManagerConfig {
	return TransactionManagerConfig{
		IsolationLevel: sql.LevelReadCommitted,
	}
}

type Transactions interface {
	Run(ctx context.Context, callback func(ctx context.Context) error) error
}

type transactions struct {
	db     *sqlx.DB
	config TransactionManagerConfig
}

func NewTransactionManager(db *sqlx.DB, config TransactionManagerConfig) Transactions {
	return &transactions{
		db:     db,
		config: config,
	}
}

func (m *transactions) Run(ctx context.Context, callback func(ctx context.Context) error) error {
	transaction, err := m.beginTransaction(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	defer func() {
		if err := recover(); err != nil {
			var panicCause error

			if e, ok := err.(error); ok {
				panicCause = e
			} else {
				panicCause = errors.New(fmt.Sprint(err))
			}

			panic(multierr.Combine(panicCause, transaction.rollback()))
		}
	}()

	if err := callback(m.putTransactionToContext(ctx, transaction)); err != nil {
		return multierr.Combine(err, transaction.rollback())
	}

	if err := transaction.commit(); err != nil {
		return multierr.Combine(err, transaction.rollback())
	}

	return nil
}

func (m *transactions) putTransactionToContext(ctx context.Context, tx *transaction) context.Context {
	return context.WithValue(ctx, txKey, tx)
}

func (m *transactions) beginTransaction(ctx context.Context) (*transaction, error) {
	if t := extractTransactionFromContext(ctx); t != nil {
		return &transaction{tx: t.tx, depth: t.depth + 1}, nil
	}

	return m.beginNewTransaction(ctx)
}

func (m *transactions) beginNewTransaction(ctx context.Context) (*transaction, error) {
	sqlxTx, err := m.db.BeginTxx(ctx, &sql.TxOptions{Isolation: m.config.IsolationLevel})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &transaction{tx: sqlxTx, depth: 0}, nil
}

type transaction struct {
	tx    *sqlx.Tx
	depth uint
}

func (t *transaction) commit() error {
	if t.depth != 0 {
		return nil
	}

	return t.tx.Commit()
}

func (t *transaction) rollback() error {
	if t.depth != 0 {
		return nil
	}

	return t.tx.Rollback()
}

func extractTransactionFromContext(ctx context.Context) *transaction {
	if tx, ok := ctx.Value(txKey).(*transaction); ok {
		return tx
	}

	return nil
}

func (d *dal) ToArgsAndExpressions(conditions map[string]interface{}) ([]interface{}, []string) {
	var args []interface{}
	var expressions []string

	for field, value := range conditions {
		if value == nil {
			expressions = append(expressions, fmt.Sprintf("%s IS NULL", field))
		} else {
			args = append(args, value)
			expressions = append(expressions, fmt.Sprintf("%s = $%d", field, len(args)))
		}
	}
	return args, expressions
}

func (d *dal) PipeErr(err error) error {
	return HandleError(err)
}

func getTransactionFromContext(ctx context.Context) *sqlx.Tx {
	tr := extractTransactionFromContext(ctx)
	if tr == nil {
		return nil
	}
	return tr.tx
}

const (
	ErrLockNotAvailable   pq.ErrorCode = "55P03"
	ErrUniqueConstraint   pq.ErrorCode = "23505"
	ErrRowCheckConstraint pq.ErrorCode = "23514"
)

func HandleError(err error) error {
	if err == nil {
		return nil
	}
	if err == sql.ErrNoRows {
		return ObjectNotFoundErr()
	}
	if driverErr, ok := err.(*pq.Error); ok {
		if driverErr.Code == ErrLockNotAvailable {
			return ObjectOnLockErr("Object is being used by another transaction")
		}
		if driverErr.Code == ErrRowCheckConstraint {
			return ObjectOnLockErr("Failed row constraint check")
		}
		if driverErr.Code == ErrUniqueConstraint {
			return ConflictErr(driverErr.Detail)
		}
		return err
	}
	logger.Error(err)
	return err
}
