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
	nestedBlocksToTag := []string{"ebs_block_device", "root_block_device"}
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
		nestedBlocks := rawBlock.Body().Blocks()
		for _, nestedBlock := range nestedBlocks {
			if inSlice(nestedBlocksToTag, nestedBlock.Type()) {
				addTraceTag(nestedBlock)
			}
		}
		addTraceTag(rawBlock)
	}
	err = os.WriteFile(filePath, hclFile.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}
	return nil
}

func addTraceTag(block *hclwrite.Block) {
	tagsAttribute := block.Body().GetAttribute("tags")
	tags := make([]hclwrite.ObjectAttrTokens, 0)
	traceAdded := false
	if tagsAttribute != nil {
		tagsTokens := tagsAttribute.Expr().BuildTokens(hclwrite.Tokens{})
		tags = parseTagAttribute(tagsTokens)
		for index, tag := range tags {
			if string(tag.Name.Bytes()) == `"cloudfix:linter_yor_trace"` {
				traceAdded = true
				tags[index].Value = hclwrite.TokensForValue(cty.StringVal(uuid.New().String()))
			}
		}
	}
	if !traceAdded {
		tags = append(tags, hclwrite.ObjectAttrTokens{
			Name:   hclwrite.TokensForValue(cty.StringVal("cloudfix:linter_yor_trace")),
			Value: hclwrite.TokensForValue(cty.StringVal(uuid.New().String())),
		})
	}
	block.Body().SetAttributeRaw("tags", hclwrite.TokensForObject(tags))
}

func findTerraformFiles(dir string) ([]string, error) {
	ignoredDirs := []string{".git", ".DS_Store", ".idea", ".terraform"}
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to scan directory %s: %s", path, err)
		}
		if !info.IsDir() && filepath.Ext(path) == ".tf" {
			for _, ignoredDir := range ignoredDirs {
				if filepath.Dir(path) == ignoredDir {
					return nil
				}
			}
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find Terraform files in directory %s: %s", dir, err)
	}
	return files, nil
}