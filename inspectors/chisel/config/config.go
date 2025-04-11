// -*- mode: go; indent-tabs-mode: t -*-

/*
 * copyright 2024 canonical ltd.
 *
 * this program is free software: you can redistribute it and/or modify
 * it under the terms of the gnu general public license version 3 as
 * published by the free software foundation.
 *
 * this program is distributed in the hope that it will be useful,
 * but without any warranty; without even the implied warranty of
 * merchantability or fitness for a particular purpose.  see the
 * gnu general public license for more details.
 *
 * you should have received a copy of the gnu general public license
 * along with this program.  if not, see <http://www.gnu.org/licenses/>.
 *
 */

package config

import (
	"github.com/canonical/fetch-service/glob"
)

type ChiselInspectorConfig struct {
	Urls []glob.Glob `yaml:"urls"`
}
