package core

import "github.com/jmoiron/sqlx"

type ModuleStorage interface {
	Transactions() Transactions
	DbConnection() *sqlx.DB
	Dal() Dal
}

type moduleStorage struct {
	transactions Transactions
	dBConnection *sqlx.DB
	dbal         Dal
}

func (m *moduleStorage) Transactions() Transactions {
	return m.transactions
}

func (m *moduleStorage) DbConnection() *sqlx.DB {
	return m.dBConnection
}

func (m *moduleStorage) Dal() Dal {
	return m.dbal
}

func NewModule(driverName, databaseDsn string) ModuleStorage {
	var m moduleStorage
	m.dBConnection = NewConnection(driverName, databaseDsn)
	m.transactions = NewTransactionManager(m.dBConnection, NewDefaultTransactionManagerConfig())
	m.dbal = NewDAL(m.dBConnection, m.transactions)

	return &m
}
