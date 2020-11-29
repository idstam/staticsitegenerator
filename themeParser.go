package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "gopkg.in/russross/blackfriday.v2"
	blackfriday "gopkg.in/russross/blackfriday.v2"
)

type ContentType struct {
	Link    string
	Title   string
	Slug    string
	Date    string
	Content string
	Draft   string
}

func renderContent(ctx map[string]string) {

	pagedContent := []string{}
	parsedContentFile := ""
	html := ""

	err := filepath.Walk(ctx["contentDir"],
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fmt.Println(path, info.Size())
			if filepath.ToSlash(path) == ctx["contentDir"] {
				return nil
			}
			if info.IsDir() {
				os.MkdirAll(getOutFilePath(path, ctx["contentDir"], ctx["outDir"]), os.ModePerm)
			} else {
				content := parseContentFile(ctx, path)
				if content.Draft == "true" && ctx["renderDrafts"] != "true" {
					return nil
				}
				outPath := ctx["outDir"] + "/" + content.Link
				os.MkdirAll(filepath.FromSlash(outPath), os.ModePerm)
				contentCtx := addContentToContext(ctx, content)
				parsedContentFile, pagedContent = parseThemeFile(contentCtx, path)
				contentCtx["contentHtml"] = contentToHtml(parsedContentFile)
				templateFile := getFirstTemplate("content.html", contentCtx["themeDir"])
				if templateFile == "" {
					log.Fatal("Found no content.html for " + path)
				}
				fmt.Print("Use template:" + templateFile)
				html, pagedContent = parseThemeFile(contentCtx, templateFile)
				writeStringToFile(outPath+"/"+"index.html", html)
			}
			return nil
		})
	if err != nil {
		log.Println(err)
	}

}

func getFirstTemplate(fileName, themePath string) string {
	dir := themePath
	for {
		fp := filepath.FromSlash(dir + "/" + fileName)
		if _, err := os.Stat(fp); !os.IsNotExist(err) {
			return fp
		}
		dir = strings.TrimSuffix(dir, filepath.Base(dir))
		dir = filepath.Dir(dir)
		dir = filepath.ToSlash(dir)

		if dir == "" {
			return ""
		}
	}

}
func renderTheme(ctx map[string]string) {

	err := filepath.Walk(filepath.FromSlash(ctx["themeDir"]),
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.Contains(path, "theme-snippets") {
				return nil
			}
			fmt.Println(path, info.Size())
			if info.IsDir() {
				os.MkdirAll(getOutFilePath(path, ctx["themeDir"], ctx["outDir"]), os.ModePerm)
			} else {
				if strings.HasSuffix(path, "index.html") {
					fileContent, pagedContent := parseThemeFile(ctx, path)
					contentFolder := strings.Replace(path, ctx["themeDir"], "", 1)
					contentFolder = strings.Replace(contentFolder, string(filepath.Separator)+"index.html", "", 1)
					contentFolder = strings.Replace(contentFolder, string(filepath.Separator), "", 1)
					if len(pagedContent) == 0 {
						outPath := getOutFilePath(path, ctx["themeDir"], ctx["outDir"])
						writeStringToFile(outPath, fileContent)
					} else {
						for pageNumber, pc := range pagedContent {
							outPath := getOutFilePath(path, ctx["themeDir"], ctx["outDir"])
							nextLink := ctx["baseUrl"] + "/" + contentFolder + "/" + strconv.Itoa(pageNumber+2) + "/index.html"
							prevLink := ctx["baseUrl"] + "/" + contentFolder + "/" + strconv.Itoa(pageNumber) + "/index.html"
							if pageNumber == 0 {
								prevLink = ctx["baseUrl"] + "/" + contentFolder + "/" + strconv.Itoa(len(pagedContent)) + "/index.html"
							}
							if pageNumber == len(pagedContent)-1 {
								nextLink = ctx["baseUrl"] + "/" + contentFolder + "/index.html"
							}
							if pageNumber == 1 {
								prevLink = ctx["baseUrl"] + "/" + contentFolder + "/index.html"
							}
							if pageNumber > 0 {
								outPath = strings.Replace(outPath, "index.html", strconv.Itoa(pageNumber+1)+string(filepath.Separator)+"index.html", 1)
							}
							os.MkdirAll(filepath.FromSlash(strings.Replace(outPath, "index.html", "", 1)), os.ModePerm)
							outStr := combinePagedContent(fileContent, pc, pageNumber+1, len(pagedContent), prevLink, nextLink)
							writeStringToFile(outPath, outStr)
						}
					}
				} else if strings.HasSuffix(path, "content.html") {
				} else {
					outPath := getOutFilePath(path, ctx["themeDir"], ctx["outDir"])
					CopyFile(path, outPath)
				}
			}
			return nil
		})
	if err != nil {
		log.Println(err)
	}

}
func combinePagedContent(pagedTemplate string, content string, pageNumber int, pageCount int, prevLink string, nextLink string) string {
	pagedTemplate = strings.Replace(pagedTemplate, "{{paged-current}}", strconv.Itoa(pageNumber), -1)
	pagedTemplate = strings.Replace(pagedTemplate, "{{paged-total}}", strconv.Itoa(pageCount), -1)
	pagedTemplate = strings.Replace(pagedTemplate, "{{paged-prev}}", prevLink, -1)
	pagedTemplate = strings.Replace(pagedTemplate, "{{paged-next}}", nextLink, -1)

	tmp := between(pagedTemplate, "{{paged-content:", "}}")
	pagedTemplate = strings.Replace(pagedTemplate, fmt.Sprintf("{{paged-content:%s}}", tmp), content, 1)

	return pagedTemplate
}

func between(value string, a string, b string) string {
	// Get substring between two strings.
	posFirst := strings.Index(value, a)
	if posFirst == -1 {
		return ""
	}
	posLast := strings.Index(value, b)
	if posLast == -1 {
		return ""
	}
	posFirstAdjusted := posFirst + len(a)
	if posFirstAdjusted >= posLast {
		return ""
	}
	return value[posFirstAdjusted:posLast]
}
func parseThemeFile(ctx map[string]string, filePath string) (string, []string) {
	outData := ""
	pagedContent := []string{}

	headerCount := 0
	file, err := os.Open(filepath.FromSlash(filePath))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		snippetData := ""

		line := scanner.Text()
		if line == "---" && headerCount < 2 {
			headerCount++
			continue
		}
		if headerCount == 2 || headerCount == 0 {
			snippetDirective := getSnippetDirective(line)
			if len(snippetDirective) > 0 {
				switch snippetDirective[0] {
				case "generator":
					line = replaceSnippet(line, templateGenerator())
				case "snippet":
					snippetData, _ = getParsedSnippet(ctx, snippetDirective)
					line = replaceSnippet(line, snippetData)
				case "context":
					snippetData = ctx[snippetDirective[1]]
					line = replaceSnippet(line, snippetData)

				case "foreach-content":
					content := getForeachContent(ctx, snippetDirective, filePath)
					line = renderForeachContent(ctx, snippetDirective, content)

				case "paged-content":
					content := getForeachContent(ctx, snippetDirective, filePath)
					pagedContent = renderPagedContent(ctx, snippetDirective, content)

				}
			}
			outData += line + "\n"
		}

	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return outData, pagedContent

}
func getSnippetDirective(line string) []string {
	if strings.Contains(line, "{{") && strings.Contains(line, "}}") {
		start := strings.Index(line, "{{") + 2
		end := strings.Index(line, "}}")
		snippet := line[start:end]
		return strings.Split(snippet, ":")
	}
	return []string{}
}

func replaceSnippet(line string, snippetData string) string {
	if strings.Contains(line, "{{") && strings.Contains(line, "}}") {
		start := strings.Index(line, "{{")
		end := strings.Index(line, "}}") + 2
		pre := line[:start]
		post := line[end:]
		return pre + snippetData + post
	}
	return line

}
func getParsedSnippet(ctx map[string]string, snippetDirective []string) (string, []string) {
	snippetFile := snippetDirective[1]
	return parseThemeFile(ctx, ctx["themeDir"]+"/theme-snippets/"+snippetFile)

}
func getForeachContent(ctx map[string]string, snippetDirective []string, filePath string) []ContentType {

	dir := filepath.Dir(filePath)
	contentDir := getContentPath(ctx, dir)
	ret := []ContentType{}
	err := filepath.Walk(contentDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fmt.Println(path, info.Size())

			if info.IsDir() {
				return nil
			} else {
				fileContent := parseContentFile(ctx, path)
				if fileContent.Draft == "true" && ctx["renderDrafts"] != "true" {
					return nil
				}
				ret = append(ret, fileContent)
			}
			return nil
		})
	if err != nil {
		log.Println(err)
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Date > ret[j].Date
	})

	return ret
}

func renderForeachContent(inCtx map[string]string, snippetDirective []string, content []ContentType) string {
	ret := ""
	foo := ""
	snippetFile := snippetDirective[1]

	for _, c := range content {
		ctx := addContentToContext(inCtx, c)

		foo, _ = parseThemeFile(ctx, ctx["themeDir"]+"/theme-snippets/"+snippetFile)
		ret = foo + "\n" + ret

	}

	return ret
}
func renderPagedContent(inCtx map[string]string, snippetDirective []string, content []ContentType) []string {
	ret := []string{}
	tmp := ""
	foo := ""
	snippetFile := snippetDirective[1]

	i := 0
	for _, c := range content {
		ctx := addContentToContext(inCtx, c)

		foo, _ = parseThemeFile(ctx, ctx["themeDir"]+"/theme-snippets/"+snippetFile)
		tmp = tmp + "\n" + foo
		i++

		if i%10 == 0 {
			ret = append(ret, tmp)
			tmp = ""
		}
	}

	ret = append(ret, tmp)

	return ret
}

func addContentToContext(inCtx map[string]string, c ContentType) map[string]string {
	ctx := map[string]string{}
	for k, v := range inCtx {
		ctx[k] = v
	}
	ctx["link"] = c.Link
	ctx["title"] = c.Title
	ctx["date"] = strings.Split(c.Date, " ")[0]
	ctx["slug"] = c.Slug
	return ctx
}
func templateGenerator() string {
	return `<meta name="generator" content="jsiStaticSites" />`
}

func getOutFilePath(themeFilePath string, themeDir, outDir string) string {
	themeFilePath = filepath.ToSlash(themeFilePath)
	themeDir = filepath.ToSlash(themeDir)
	outDir = filepath.ToSlash(outDir)
	ret := strings.Replace(themeFilePath, themeDir, outDir, 1)
	return filepath.FromSlash(ret)
}
func getContentPath(ctx map[string]string, themePath string) string {
	themePath = filepath.ToSlash(themePath)
	return filepath.FromSlash(strings.Replace(themePath, filepath.ToSlash(ctx["themeDir"]), filepath.ToSlash(ctx["contentDir"]), 1))
}

func writeStringToFile(filePath string, content string) {
	//fmt.Println(content)

	fo, err := os.Create(filepath.FromSlash(filePath))
	if err != nil {
		log.Fatal(err)
	}
	defer fo.Close()

	_, err = io.Copy(fo, strings.NewReader(content))
	if err != nil {
		log.Fatal(err)
	}

}

func contentToHtml(content string) string {
	html := blackfriday.Run([]byte(content))
	return string(html)
}

func parseContentFile(ctx map[string]string, filePath string) ContentType {
	ret := ContentType{}
	headerCount := 0

	file, err := os.Open(filepath.FromSlash(filePath))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	foo := strings.Replace(filePath, filepath.Ext(filePath), "", 1)
	foo = filepath.ToSlash(foo)
	foo = strings.Replace(foo, filepath.ToSlash(ctx["contentDir"]), "", 1)
	ret.Link = foo

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "---" && headerCount < 2 {
			headerCount++
			continue
		}
		if headerCount < 2 {
			if strings.HasPrefix(line, "title:") {
				ret.Title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
			}
			if strings.HasPrefix(line, "slug:") {
				ret.Slug = strings.TrimSpace(strings.TrimPrefix(line, "slug:"))
			}
			if strings.HasPrefix(line, "date:") {
				ret.Date = strings.TrimSpace(strings.TrimPrefix(line, "date:"))
			}
			if strings.HasPrefix(line, "draft:") {
				ret.Draft = strings.TrimSpace(strings.TrimPrefix(line, "draft:"))
			}
		} else {
			ret.Content += line + "\n"
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return ret
}
