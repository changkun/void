// Copyright (c) 2021 Changkun Ou <hi@changkun.de>. All Rights Reserved.
// Unauthorized using, copying, modifying and distributing, via any
// medium is strictly prohibited.

// This program initializes a bbolt database for the void.
package main

import (
	"fmt"

	"go.etcd.io/bbolt"
)

func main() {
	db, err := bbolt.Open("void.db", 0666, nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket([]byte("files"))
		if err != nil {
			return fmt.Errorf("cannot create bucket: %s", err)
		}
		_, err = tx.CreateBucket([]byte("temps"))
		if err != nil {
			return fmt.Errorf("cannot create bucket: %s", err)
		}
		return nil
	})
}
