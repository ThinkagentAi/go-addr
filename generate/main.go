package main

import "github.com/ThinkagentAi/goaddr/generate/autoCode"

//go:generate go run main.go
func main() {
	autoCode.AutoAreaMap()
}
