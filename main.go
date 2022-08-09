package main

import (
	"context"
	"flag"
	"fmt"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"log"
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
		log.Fatalln("no files found")
	} else if len(response.Files) > 1 {
		log.Fatalln("too many files found")
	}

	q = fmt.Sprintf("name='%s.zip' and '%s' in parents and mimeType!='application/vnd.google-apps.folder'", version, response.Files[0].Id)
	response, err = svc.Files.List().Context(ctx).Q(q).Do()
	if err != nil {
		log.Fatalf("failed to list files: %v", err)
	}

	for _, v := range response.Files {
		log.Println(v.Name, v.Id)
	}
}
