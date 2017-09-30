package main

import (
	"fmt"

	"./utils"
)

func main() {
	count := 0
	for count >= 0 {
		fmt.Printf("count: %d\n", count)
		utils.IncIntPtr(&count)
	}
}
