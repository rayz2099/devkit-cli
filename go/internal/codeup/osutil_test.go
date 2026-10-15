package codeup

import "os"

func osStat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func osNotExist(err error) bool {
	return os.IsNotExist(err)
}
