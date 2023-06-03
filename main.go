package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

func inSlice[T comparable](elems []T, v T) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}

func getUncloseBracketsCount(bracketsCounters map[hclsyntax.TokenType]int) int {
	sum := 0
	for b := range bracketsCounters {
		sum += bracketsCounters[b]
	}

	return sum
}

func extractTagPairs(tokens hclwrite.Tokens) []hclwrite.Tokens {
	separatorTokens := []hclsyntax.TokenType{hclsyntax.TokenComma, hclsyntax.TokenNewline}

	bracketsCounters := map[hclsyntax.TokenType]int{
		hclsyntax.TokenOParen: 0,
		hclsyntax.TokenOBrack: 0,
	}

	openingBrackets := []hclsyntax.TokenType{hclsyntax.TokenOParen, hclsyntax.TokenOBrack}
	closingBrackets := []hclsyntax.TokenType{hclsyntax.TokenCParen, hclsyntax.TokenCBrack}

	bracketsPairs := map[hclsyntax.TokenType]hclsyntax.TokenType{
		hclsyntax.TokenCParen: hclsyntax.TokenOParen,
		hclsyntax.TokenCBrack: hclsyntax.TokenOBrack,
	}

	tagPairs := make([]hclwrite.Tokens, 0)
	startIndex := 0
	hasEq := false
	for i, token := range tokens {
		if inSlice(separatorTokens, token.Type) && getUncloseBracketsCount(bracketsCounters) == 0 {
			if hasEq {
				tagPairs = append(tagPairs, tokens[startIndex:i])
			}
			startIndex = i + 1
			hasEq = false
		}
		if token.Type == hclsyntax.TokenEqual {
			hasEq = true
		}
		if inSlice(openingBrackets, token.Type) {
			bracketsCounters[token.Type]++
		}
		if inSlice(closingBrackets, token.Type) {
			matchingOpen := bracketsPairs[token.Type]
			bracketsCounters[matchingOpen]--
		}
	}
	if hasEq {
		tagPairs = append(tagPairs, tokens[startIndex:])
	}

	return tagPairs
}

func getHclMapsContents(tokens hclwrite.Tokens) []hclwrite.Tokens {
	hclMaps := make([]hclwrite.Tokens, 0)
	bracketOpenIndex := -1

	for i, token := range tokens {
		if token.Type == hclsyntax.TokenOBrace {
			bracketOpenIndex = i
		}
		if token.Type == hclsyntax.TokenCBrace {
			hclMaps = append(hclMaps, tokens[bracketOpenIndex+1:i])
		}
	}

	return hclMaps
}

func parseTagAttribute(tokens hclwrite.Tokens) map[string]string {
	hclMaps := getHclMapsContents(tokens)
	tagPairs := make([]hclwrite.Tokens, 0)
	for _, hclMap := range hclMaps {
		tagPairs = append(tagPairs, extractTagPairs(hclMap)...)
	}
	
	parsedTags := make(map[string]string)
	for _, entry := range tagPairs {
		eqIndex := -1
		var key string
		for j, token := range entry {
			if token.Type == hclsyntax.TokenEqual {
				eqIndex = j + 1
				key = strings.TrimSpace(string(entry[:j].Bytes()))
			}
		}
		value := string(entry[eqIndex:].Bytes())
		value = strings.TrimPrefix(strings.TrimSuffix(value, " "), " ")
		_ = json.Unmarshal([]byte(key), &key)
		_ = json.Unmarshal([]byte(value), &value)
		parsedTags[key] = value
	}

	return parsedTags
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide the directory path as a command-line argument.")
		return
	}

	dir := os.Args[1]
	files, err := findTerraformFiles(dir)
	if err != nil {
		fmt.Printf("Failed to find Terraform files: %s\n", err)
		return
	}

	for _, file := range files {
		err := addTags(file)
		if err != nil {
			fmt.Printf("Failed to add tags to file %s: %s\n", file, err)
		}
	}
}

func addTags(filePath string) error {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	hclFile, diagnostics := hclwrite.ParseConfig(src, filePath, hcl.InitialPos)
	if diagnostics.HasErrors() {
		hclErrors := diagnostics.Errs()
		return fmt.Errorf("failed to parse file: %v", hclErrors)
	}

	rawBlocks := hclFile.Body().Blocks()
	for _, rawBlock := range rawBlocks {
		if rawBlock.Type() != "resource" {
			continue
		}

		nested := rawBlock.Body().Blocks()
		for _, innerBlock := range nested {
			tagsMap := make(map[string]cty.Value, 1)
			tagsAttribute := innerBlock.Body().GetAttribute("tags")

			if tagsAttribute != nil {
				tagsTokens := tagsAttribute.Expr().BuildTokens(hclwrite.Tokens{})
				parsedTags := parseTagAttribute(tagsTokens)
				for key := range parsedTags {
					tagsMap[key] = cty.StringVal(parsedTags[key])
				}
			}
			tagsMap["cloudfix:linter_yor_trace"] = cty.StringVal(uuid.New().String())
			innerBlock.Body().SetAttributeRaw("tags", hclwrite.TokensForValue(cty.MapVal(tagsMap)))
		}

		tagsMap := make(map[string]cty.Value, 1)
		tagsAttribute := rawBlock.Body().GetAttribute("tags")

		if tagsAttribute != nil {
			tagsTokens := tagsAttribute.Expr().BuildTokens(hclwrite.Tokens{})
			parsedTags := parseTagAttribute(tagsTokens)
			for key := range parsedTags {
				tagsMap[key] = cty.StringVal(parsedTags[key])
			}
		}
		tagsMap["cloudfix:linter_yor_trace"] = cty.StringVal(uuid.New().String())
		rawBlock.Body().SetAttributeRaw("tags", hclwrite.TokensForValue(cty.MapVal(tagsMap)))
	}

	err = os.WriteFile(filePath, hclFile.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

func findTerraformFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to scan directory %s: %s", path, err)
		}
		if !info.IsDir() && filepath.Ext(path) == ".tf" {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to find Terraform files in directory %s: %s", dir, err)
	}

	return files, nil
}