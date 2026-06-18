package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

const tokenInputPath = "testdata/token_inputs.txt"

// main
func main() {
	textList, modelList, encodingList, err := readTestFile()
	if err != nil {
		log.Fatal(err)
	}
	testTokenByModel(textList, modelList)
	fmt.Println("=========================================")
	testTokenByEncoding(textList, encodingList)
}

// read all columns from a file
func readTestFile() ([]string, []string, []string, error) {
	file, err := os.Open(tokenInputPath)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = file.Close() }()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, nil, err
	}
	textList := strings.Split(lines[0], ",")
	modelList := strings.Split(lines[1], ",")
	encodingList := strings.Split(lines[2], ",")

	return textList, modelList, encodingList, nil
}

// getTokenByModel
func getTokenByModel(text, model string) int {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		return 0
	}

	token := tkm.Encode(text, nil, nil)

	return len(token)
}

// getTokenByEncoding
func getTokenByEncoding(text, encoding string) int {
	tke, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return 0
	}

	token := tke.Encode(text, nil, nil)

	return len(token)
}

// testTokenByModel
func testTokenByModel(textList, modelList []string) {
	for i := range textList {
		for j := range modelList {
			fmt.Printf("text: %s, model: %s, token: %d\n", textList[i], modelList[j], getTokenByModel(textList[i], modelList[j]))
		}
	}
}

// testTokenByEncoding
func testTokenByEncoding(textList, encodingList []string) {
	for i := range textList {
		for j := range encodingList {
			fmt.Printf("text: %s, encoding: %s, token: %d\n", textList[i], encodingList[j], getTokenByEncoding(textList[i], encodingList[j]))
		}
	}
}
