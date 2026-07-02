package main

import "os"

func main() {
	os.Exit(1) // want "direct os.Exit call in main function is forbidden"
}

func shutdown() {
	os.Exit(1)
}
