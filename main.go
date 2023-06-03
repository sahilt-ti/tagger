package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

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
			if innerBlock.Type() != "ebs_block_device" &&
				innerBlock.Type() != "root_block_device" {
				continue
			}
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