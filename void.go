// Copyright (c) 2021 Changkun Ou <hi@changkun.de>. All Rights Reserved.
// Unauthorized using, copying, modifying and distributing, via any
// medium is strictly prohibited.

// It is required to configure the following environment variables
// to use this program.
// - VOID_PORT
// - VOID_TG_BOTTOKEN
// - VOID_TG_CHATID
// - VOID_DB
// - VOID_USER
// - VOID_PASS
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"changkun.de/x/void/internal/cmd"
	"changkun.de/x/void/internal/void"
)

func main() {
	log.SetPrefix("void: ")
	log.SetFlags(0)

	flag.CommandLine.Usage = func() {
		fmt.Fprintf(os.Stderr, `void is a zero storage cost file system.
Open sourced at https://changkun.de/s/void.

Command line usage:
$ void up PATH [, PATH...]
$ void down ID [, ID...]
$ void del ID [, ID...]
$ void ls
$ void serv
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.CommandLine.Usage()
		return
	}

	void.LoadConf()

	switch args[0] {
	case "up", "upload":
		for _, path := range args[1:] {
			_, file := filepath.Split(path)
			r, err := cmd.Upload(path)
			if err != nil {
				log.Printf("%s: %v\n", file, err)
				return
			}
			log.Printf("%s: %s?id=%s\n", file, cmd.Endpoint, r.Id)
		}
	case "down", "download":
		for _, id := range args[1:] {
			err := cmd.Download(id)
			if err != nil {
				log.Printf("%s: %v\n", id, err)
			}
		}
	case "del", "delete", "rm", "remove":
		for _, id := range args[1:] {
			err := cmd.Delete(id)
			if err != nil {
				log.Printf("%s: %v\n", id, err)
			}
			log.Printf("%s: DONE.\n", id)
		}
	case "ls", "list":
		files, err := cmd.List()
		if err != nil {
			log.Printf("%v\n", err)
		}

		log.Println("Id\tFileName\tFileSize\tUploadId")
		for _, file := range files {
			log.Println(file)
		}
	case "serv", "serve":
		void.NewServer().Run()
	default:
		flag.CommandLine.Usage()
	}
}
