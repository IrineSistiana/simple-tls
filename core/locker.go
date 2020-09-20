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

type locker struct {
	c chan struct{}
}

func newLocker() *locker {
	l := &locker{
		c: make(chan struct{}, 1),
	}
	l.c <- struct{}{}
	return l
}

func (l *locker) lock() {
	<-l.c
}

func (l *locker) tryLock() bool {
	select {
	case <-l.c:
		return true
	default:
		return false
	}
}

func (l *locker) unlock() {
	select {
	case l.c <- struct{}{}:
	default:
		panic("locker: unlocked twice")
	}
}
