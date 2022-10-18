//     Copyright (C) 2020-2021, IrineSistiana
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

package mlog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net"
	"os"
)

func LogConnErr(msg string, conn net.Conn, err error) {
	logger.Error(msg, zap.Stringer("remote", conn.RemoteAddr()), zap.Stringer("local", conn.LocalAddr()), zap.Error(err))
}

var logLvl = zap.NewAtomicLevelAt(zap.InfoLevel)

var logger = initLogger()

func L() *zap.Logger {
	return logger
}

func SetLvl(l zapcore.Level) {
	logLvl.SetLevel(l)
}

func initLogger() *zap.Logger {
	ec := zapcore.EncoderConfig{
		TimeKey:        "time",
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	core := zapcore.NewCore(zapcore.NewConsoleEncoder(ec), zapcore.Lock(os.Stderr), logLvl)
	return zap.New(core)
}
