// Copyright (c) 2021 Changkun Ou <hi@changkun.de>. All Rights Reserved.
// Unauthorized using, copying, modifying and distributing, via any
// medium is strictly prohibited.

//go:build !prod

package cmd

import "changkun.de/x/void/internal/void"

var Endpoint = "http://0.0.0.0" + void.Conf.Port + "/void"
