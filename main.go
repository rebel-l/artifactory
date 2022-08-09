package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"log"
	"sort"
	"time"
)

func main() {
	ctx := context.Background()

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

	q = fmt.Sprintf("name='%s.zip' and '%s' in parents and mimeType!='application/vnd.google-apps.folder'", version, response.Files[0].Id)
	response, err = svc.Files.List().Context(ctx).Q(q).Fields("nextPageToken", "files(id, name, createdTime)").Do()
	if err != nil {
		log.Fatalf("failed to list files: %v", err)
	}

	if response.NextPageToken != "" {
		log.Fatalf("too many files for version %s found", version)
	}

	_ = sort.Reverse(ByCreatedTime(response.Files))

	for _, v := range response.Files {
		log.Println(v.Name, v.Id, v.CreatedTime)
	}
}

type ByCreatedTime []*drive.File

func (b ByCreatedTime) Len() int      { return len(b) }
func (b ByCreatedTime) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b ByCreatedTime) Less(i, j int) bool {
	first, _ := time.Parse(time.RFC3339, b[i].CreatedTime)
	second, _ := time.Parse(time.RFC3339, b[j].CreatedTime)

	return first.Before(second)
}
