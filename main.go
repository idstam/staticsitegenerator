package main

import (
	"bufio"
	"log"
	"os"
	"strings"

	"github.com/otiai10/copy"
	_ "github.com/otiai10/copy"
)

func main() {

	ctx := readContext()

	err := os.RemoveAll(ctx["outDir"] + "_old")
	if err != nil {
		log.Fatal(err.Error())
	}
	os.Rename(ctx["outDir"], ctx["outDir"]+"_old")
	if err != nil {
		log.Fatal(err.Error())
	}
	os.Mkdir(ctx["outDir"], os.ModePerm)
	if err != nil {
		log.Fatal(err.Error())
	}

	renderContent(ctx)
	renderTheme(ctx)
	copyStatics(ctx)

}

func readContext() map[string]string {
	file, err := os.Open("context.cfg")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	ctx := map[string]string{}
	curKey := ""
	curVal := ""

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ":") {
			if curKey != "" {
				ctx[curKey] = strings.TrimSpace(curVal)
			}
			curVal = ""
			curKey = strings.TrimPrefix(line, ":")
			curKey = strings.TrimSpace(curKey)
		} else {
			curVal += line + "\n"
		}
	}

	ctx[curKey] = strings.TrimSpace(curVal)

	return ctx
}

func copyStatics(ctx map[string]string) {

	s := ctx["staticDir"]
	o := ctx["outDir"]
	err := copy.Copy(s, o)
	if err != nil {
		log.Fatal(err)
	}
}
