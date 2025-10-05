package main

import (
	"bilinovel-downloader/cmd"
	"io"
	"log"

	"github.com/playwright-community/playwright-go"
)

func main() {
	log.Println("Installing playwright")
	err := playwright.Install(&playwright.RunOptions{
		Browsers: []string{"chromium"},
		Stdout:   io.Discard,
	})
	if err != nil {
		log.Panicf("failed to install playwright")
	}
	_ = cmd.RootCmd.Execute()
}
