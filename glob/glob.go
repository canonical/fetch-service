// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024 Canonical Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package glob

import (
	"github.com/gobwas/glob"
)

// Glob is a YAML-unmarshable glob pattern type.
type Glob struct {
	G glob.Glob
}

func MustCompile(pattern string) Glob {
	return Glob{G: glob.MustCompile(pattern, '/')}
}

func (t *Glob) UnmarshalYAML(unmarshal func(v interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	g, err := glob.Compile(s, '/')
	if err != nil {
		return err
	}

	*t = Glob{g}
	return nil
}

func (t *Glob) Match(s string) bool {
	return t.G.Match(s)
}
