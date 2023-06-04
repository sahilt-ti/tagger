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
type Tag struct {
	Key   string
	Value string
}

var ignoredDirs = []string{".git", ".DS_Store", ".idea", ".terraform"}

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
		nestedBlocks := rawBlock.Body().Blocks()
		for _, nestedBlock := range nestedBlocks {
			if nestedBlock.Type() != "ebs_block_device" &&
				nestedBlock.Type() != "root_block_device" {
				continue
			}
			tagsAttribute := nestedBlock.Body().GetAttribute("tags")
			
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
			nestedBlock.Body().SetAttributeRaw("tags", hclwrite.TokensForObject(tags))
		}
		
		tagsAttribute := rawBlock.Body().GetAttribute("tags")
		
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
		rawBlock.Body().SetAttributeRaw("tags", hclwrite.TokensForObject(tags))
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