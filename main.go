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

// TODO: would be nice to upload executable with github actions on tagging
func main() {
	var credentialFile string

	ctx := context.Background()

	o := NewOptions()

	flag.StringVar(&o.application, "a", "", "name of the application (mandatory)")
	flag.StringVar(&credentialFile, "c", "", "path and name of file with google credentials (mandatory)")
	flag.StringVar(&o.dst, "d", o.dst, "path to the destination")
	flag.StringVar(&o.version, "v", "", "version of the application (mandatory)")
	flag.Parse()

	if credentialFile == "" || !o.IsValid() {
		flag.Usage()
		return
	}

	svc, err := drive.NewService(ctx, option.WithCredentialsFile(credentialFile))
	if err != nil {
		log.Fatalf("failed to init google drive service: %v", err)
	}

	if err = do(ctx, svc, o); err != nil {
		log.Fatalf("failed to get artifact: %v", err)
	}

	log.Println("artifact successfully downloaded!")
}

func do(ctx context.Context, svc *drive.Service, o options) error {
	log.Println("find folder ...")

	q := fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder'", o.application)
	response, err := svc.Files.List().Context(ctx).Q(q).Do()
	if err != nil {
		return fmt.Errorf("failed to find folder: %w", err)
	}

	if len(response.Files) == 0 {
		return fmt.Errorf("no files found")
	} else if len(response.Files) > 1 {
		return fmt.Errorf("too many files found")
	}

	log.Println("find files ...")

	q = fmt.Sprintf("name='%s.zip' and '%s' in parents and mimeType!='application/vnd.google-apps.folder'", o.version, response.Files[0].Id)
	response, err = svc.Files.List().Context(ctx).Q(q).Fields("nextPageToken", "files(id, name, createdTime)").Do()
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	if response.NextPageToken != "" {
		return fmt.Errorf("too many files for version %s found", o.version)
	} else if len(response.Files) <= 0 {
		return fmt.Errorf("no files for version %s found", o.version)
	}

	_ = sort.Reverse(ByCreatedTime(response.Files))

	log.Println("download latest artifact ...")

	artifact, err := svc.Files.Get(response.Files[0].Id).Context(ctx).Download()
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	log.Println("start to unzip artifact ...")

	body, err := io.ReadAll(artifact.Body)
	if err != nil {
		return fmt.Errorf("failed to get body: %w", err)
	}
	_ = artifact.Body.Close()

	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("failed to read zip archive: %w", err)
	}

	// TODO: progress bar

	for _, v := range archive.File {
		file := filepath.Join(o.dst, v.Name)
		fmt.Println("unzipping file ", file)

		if !strings.HasPrefix(file, filepath.Clean(o.dst)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path")
		}

		if v.FileInfo().IsDir() {
			fmt.Println("creating directory...")
			if err := os.MkdirAll(file, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory '%s': %w", file, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(file), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory '%s': %w", file, err)
		}

		dstFile, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, v.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file '%s': %w", file, err)
		}

		fileInArchive, err := v.Open()
		if err != nil {
			return fmt.Errorf("failed to open file '%s' of archive: %w", v.Name, err)
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			return fmt.Errorf("failed to copy content of file '%s': %w", v.Name, err)
		}

		_ = dstFile.Close()
		_ = fileInArchive.Close()
	}

	return nil
}

type ByCreatedTime []*drive.File

func (b ByCreatedTime) Len() int      { return len(b) }
func (b ByCreatedTime) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b ByCreatedTime) Less(i, j int) bool {
	first, _ := time.Parse(time.RFC3339, b[i].CreatedTime)
	second, _ := time.Parse(time.RFC3339, b[j].CreatedTime)

	return first.Before(second)
}

type options struct {
	dst         string
	application string
	version     string
}

func (o options) IsValid() bool {
	return o.application != "" && o.version != "" && o.dst != ""
}

func NewOptions() options {
	return options{dst: "output"}
}
