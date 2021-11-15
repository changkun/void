// Copyright (c) 2021 Changkun Ou <hi@changkun.de>. All Rights Reserved.
// Unauthorized using, copying, modifying and distributing, via any
// medium is strictly prohibited.

package void

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var errUnauthorized = errors.New("request unauthorized")

// blocklist holds the ip that should be blocked for further requests.
//
// This map may keep grow without releasing memory because of
// continuously attempts. we also do not persist this type of block info
// to the disk, which means if we reboot the service then all the blocker
// are gone and they can attack the server again.
// We clear the map very month.
var blocklist sync.Map // map[string]*blockinfo{}

func init() {
	t := time.NewTicker(time.Hour * 24 * 30)
	go func() {
		for range t.C {
			blocklist.Range(func(k, v interface{}) bool {
				blocklist.Delete(k)
				return true
			})
		}
	}()
}

type blockinfo struct {
	failCount int64
	lastFail  atomic.Value // time.Time
	blockTime atomic.Value // time.Duration
}

const maxFailureAttempts = 3

func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) (user, pass string, err error) {
	w.Header().Set("WWW-Authenticate", `Basic realm="void"`)

	u, p, ok := r.BasicAuth()
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		err = fmt.Errorf("%w: failed to parsing basic auth", errUnauthorized)
		return
	}

	// check if the IP failure attempts are too much
	// if so, direct abort the request without checking credentials
	ip := readIP(r)
	if i, ok := blocklist.Load(ip); ok {
		info := i.(*blockinfo)
		count := atomic.LoadInt64(&info.failCount)
		if count > maxFailureAttempts {
			// if the ip is under block, then directly abort
			last := info.lastFail.Load().(time.Time)
			bloc := info.blockTime.Load().(time.Duration)

			if time.Now().UTC().Sub(last.Add(bloc)) < 0 {
				log.Printf("block ip %v, too much failure attempts. Block time: %v, release until: %v\n",
					ip, bloc, last.Add(bloc))
				err = fmt.Errorf("%w: too much failure attempts", errUnauthorized)
				return
			}

			// clear the failcount, but increase the next block time
			atomic.StoreInt64(&info.failCount, 0)
			info.blockTime.Store(bloc * 2)
		}
	}

	defer func() {
		if !errors.Is(err, errUnauthorized) {
			return
		}

		if i, ok := blocklist.Load(ip); !ok {
			info := &blockinfo{
				failCount: 1,
			}
			info.lastFail.Store(time.Now().UTC())
			info.blockTime.Store(time.Second * 10)

			blocklist.Store(ip, info)
		} else {
			info := i.(*blockinfo)
			atomic.AddInt64(&info.failCount, 1)
			info.lastFail.Store(time.Now().UTC())
		}
	}()

	if !(u == Conf.Auth.Username && p == Conf.Auth.Password) {
		w.WriteHeader(http.StatusUnauthorized)
		return "", "", fmt.Errorf("%w: username or password is invalid", errUnauthorized)
	}
	return u, p, nil
}
