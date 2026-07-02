package main

import process "os"

func main() {
	process.Exit(1) // want "direct os.Exit call in main function is forbidden"
}
