//     Copyright (C) 2020, IrineSistiana
//
//     This file is part of simple-tls.
//
//     simple-tls is free software: you can redistribute it and/or modify
//     it under the terms of the GNU General Public License as published by
//     the Free Software Foundation, either version 3 of the License, or
//     (at your option) any later version.
//
//     simple-tls is distributed in the hope that it will be useful,
//     but WITHOUT ANY WARRANTY; without even the implied warranty of
//     MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//     GNU General Public License for more details.
//
//     You should have received a copy of the GNU General Public License
//     along with this program.  If not, see <https://www.gnu.org/licenses/>.

package core

import (
	"errors"
	"net"
	"os"
	"strings"
)

var ErrBrokenSIP003Args = errors.New("invalid SIP003 args")

//SIP003Args contains sip003 args
type SIP003Args struct {
	SS_REMOTE_HOST    string
	SS_REMOTE_PORT    string
	SS_LOCAL_HOST     string
	SS_LOCAL_PORT     string
	SS_PLUGIN_OPTIONS SpoArgs
}

type SpoArgs map[string]string

func (args *SIP003Args) GetRemoteAddr() string {
	return net.JoinHostPort(args.SS_REMOTE_HOST, args.SS_REMOTE_PORT)
}

func (args *SIP003Args) GetLocalAddr() string {
	return net.JoinHostPort(args.SS_LOCAL_HOST, args.SS_LOCAL_PORT)
}

//GetSIP003Args get sip003 args from os.Environ(), if no args, returns nil
func GetSIP003Args() (*SIP003Args, error) {
	srh, srhOk := os.LookupEnv("SS_REMOTE_HOST")
	srp, srpOk := os.LookupEnv("SS_REMOTE_PORT")
	slh, slhOk := os.LookupEnv("SS_LOCAL_HOST")
	slp, slpOk := os.LookupEnv("SS_LOCAL_PORT")
	spo, spoOk := os.LookupEnv("SS_PLUGIN_OPTIONS")

	if srhOk || srpOk || slhOk || slpOk || spoOk { // has at least one arg
		if !(srhOk && srpOk && slhOk && slpOk) { // but not has all 4 args
			return nil, ErrBrokenSIP003Args
		}
	} else {
		return nil, nil // can't find any sip003 arg
	}

	return &SIP003Args{
		SS_REMOTE_HOST:    srh,
		SS_REMOTE_PORT:    srp,
		SS_LOCAL_HOST:     slh,
		SS_LOCAL_PORT:     slp,
		SS_PLUGIN_OPTIONS: formatSSPluginOptions(spo),
	}, nil
}

func formatSSPluginOptions(spo string) SpoArgs {
	spoArgs := make(SpoArgs)
	op := strings.Split(spo, ";")
	for _, so := range op {
		optionPair := strings.SplitN(so, "=", 2)
		switch len(optionPair) {
		case 1:
			spoArgs[optionPair[0]] = ""
		case 2:
			spoArgs[optionPair[0]] = optionPair[1]
		default:
			panic("unexpected pair size")
		}
	}

	return spoArgs
}
