package core

import "os"

func Mkdir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}
