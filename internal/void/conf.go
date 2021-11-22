// Copyright (c) 2021 Changkun Ou <hi@changkun.de>. All Rights Reserved.
// Unauthorized using, copying, modifying and distributing, via any
// medium is strictly prohibited.

package void

import (
	"errors"
	"flag"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"changkun.de/x/login"
)

type config struct {
	Port     string
	BotToken string
	ChatID   int64
	DB       string
	Auth     string
	SSO      string
}

var Conf config

func LoadConf() {
	isServer := false
	if flag.Args()[0] == "serv" {
		isServer = true
	}

	var err error
	if isServer {
		Conf.Port = os.Getenv("VOID_PORT")
		if !strings.HasPrefix(Conf.Port, ":") {
			log.Fatalf(`VOID_PORT has no ":" prefix`)
		}
		_, err = strconv.ParseInt(strings.TrimPrefix(Conf.Port, ":"), 10, 0)
		if err != nil {
			log.Fatalf(`VOID_PORT contains invalid digits after ":", expect eg. ":8088", got %s`, Conf.Port)
		}
		Conf.DB, err = filepath.Abs(os.Getenv("VOID_DB"))
		if err != nil {
			log.Fatalf("invalid VOID_DB location: %s", Conf.DB)
		}
		_, err = os.Stat(Conf.DB)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				log.Fatalf("VOID_DB file does not exist: %s", Conf.DB)
			}
			log.Fatalf("VOID_DB is not a valid db file: %v", err)
		}
		if !strings.HasSuffix(Conf.DB, ".db") {
			log.Fatalf("VOID_DB refers to a non .db file: %s", Conf.DB)
		}
	} else {
		username := os.Getenv("VOID_USER")
		password := os.Getenv("VOID_PASS")
		if username == "" || password == "" {
			log.Fatalf("VOID_USER or VOID_PASS is empty!")
		}
		Conf.Auth, err = login.RequestToken(username, password)
		if err != nil {
			log.Fatalf("cannot login into the void system")
		}
	}

	Conf.BotToken = os.Getenv("VOID_TG_BOTTOKEN")
	if Conf.BotToken == "" {
		log.Fatalf("missing VOID_TG_BOTTOKEN.")
	}
	Conf.ChatID, err = strconv.ParseInt(os.Getenv("VOID_TG_CHATID"), 10, 0)
	if err != nil {
		log.Fatalf("VOID_TG_CHATID is not an integer")
	}
	Conf.SSO = os.Getenv("VOID_LOGIN")
	if Conf.SSO == "" {
		log.Fatalf("missing VOID_LOGIN endpoint")
	}
}
