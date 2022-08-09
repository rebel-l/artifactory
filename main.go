package main

import (
	"archive/zip"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func main() {
	ctx := context.Background()

	dst := "output"

	var credentialFile, application, version string
	flag.StringVar(&application, "a", "", "name of the application (mandatory)")
	flag.StringVar(&credentialFile, "c", "", "path and name of file with google credentials (mandatory)")
	flag.StringVar(&version, "v", "", "version of the application (mandatory)")
	flag.Parse()

	if credentialFile == "" || application == "" || version == "" {
		flag.Usage()
		return
	}

	svc, err := drive.NewService(ctx, option.WithCredentialsFile(credentialFile))
	if err != nil {
		log.Fatalf("failed to init google drive service: %v", err)
	}

	log.Println("find folder ...")

	q := fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder'", application)
	response, err := svc.Files.List().Context(ctx).Q(q).Do()
	if err != nil {
		log.Fatalf("failed to find folder: %v", err)
	}

	if len(response.Files) == 0 {
		log.Fatal("no files found")
	} else if len(response.Files) > 1 {
		log.Fatal("too many files found")
	}

	log.Println("find files ...")

	q = fmt.Sprintf("name='%s.zip' and '%s' in parents and mimeType!='application/vnd.google-apps.folder'", version, response.Files[0].Id)
	response, err = svc.Files.List().Context(ctx).Q(q).Fields("nextPageToken", "files(id, name, createdTime)").Do()
	if err != nil {
		log.Fatalf("failed to list files: %v", err)
	}

	if response.NextPageToken != "" {
		log.Fatalf("too many files for version %s found", version)
	} else if len(response.Files) <= 0 {
		log.Fatalf("no files for version %s found", version)
	}

	_ = sort.Reverse(ByCreatedTime(response.Files))

	log.Println("download latest artifact ...")

	artifact, err := svc.Files.Get(response.Files[0].Id).Context(ctx).Download()
	if err != nil {
		log.Fatalf("failed to download file: %v", err)
	}

	log.Println("start to unzip artifact ...")

	body, err := io.ReadAll(artifact.Body)
	if err != nil {
		log.Fatalf("failed to get body: %v", err)
	}
	_ = artifact.Body.Close()

	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		log.Fatalf("failed to read zip archive: %v", err)
	}

	// TODO: progress bar

	for _, v := range archive.File {
		file := filepath.Join(dst, v.Name)
		fmt.Println("unzipping file ", file)

		if !strings.HasPrefix(file, filepath.Clean(dst)+string(os.PathSeparator)) {
			fmt.Println("invalid file path")
			return
		}

		if v.FileInfo().IsDir() {
			fmt.Println("creating directory...")
			if err := os.MkdirAll(file, os.ModePerm); err != nil {
				log.Fatalf("failed to create directory '%s': %v", file, err) // TODO: move all code with log.Fatal into subroutine which returns proper error
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(file), os.ModePerm); err != nil {
			log.Fatalf("failed to create directory '%s': %v", file, err)
		}

		dstFile, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, v.Mode())
		if err != nil {
			log.Fatalf("failed to create file '%s': %v", file, err)
		}

		fileInArchive, err := v.Open()
		if err != nil {
			log.Fatalf("failed to open file '%s' of archive: %v", v.Name, err)
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			log.Fatalf("failed to copy content of file '%s': %v", v.Name, err)
		}

		_ = dstFile.Close()
		_ = fileInArchive.Close()
	}

	log.Println("artifact successfully downloaded!")
}

type ByCreatedTime []*drive.File

func (b ByCreatedTime) Len() int      { return len(b) }
func (b ByCreatedTime) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b ByCreatedTime) Less(i, j int) bool {
	first, _ := time.Parse(time.RFC3339, b[i].CreatedTime)
	second, _ := time.Parse(time.RFC3339, b[j].CreatedTime)

	return first.Before(second)
}
