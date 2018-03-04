package app

import (
	"os"
)

func initLogFile(filename string) *os.File {
	f, err := os.Create(filename)
	if err != nil {
		if os.ErrExist.Error() == err.Error() {
			f, err = os.Open(filename)
			if err != nil {
				panic(err)
			}
			return f
		}
		panic(err)
	}
	return f
}
