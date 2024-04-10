// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2024 Canonical Ltd.
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

package opinions

import (
	"errors"
)

type OpinionKind int

const (
	Unknown OpinionKind = iota
	Rejected
	Approved
	Pending
)

func (t OpinionKind) MarshalJSON() ([]byte, error) {
	switch t {
	case Unknown:
		return []byte(`"Unknown"`), nil
	case Rejected:
		return []byte(`"Rejected"`), nil
	case Approved:
		return []byte(`"Approved"`), nil
	case Pending:
		return []byte(`"Pending"`), nil
	default:
		return nil, errors.New("invalid opinion kind")
	}
}

func (t *OpinionKind) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"Unknown"`:
		*t = Unknown
		return nil
	case `"Rejected"`:
		*t = Rejected
		return nil
	case `"Approved"`:
		*t = Approved
		return nil
	case `"Pending"`:
		*t = Pending
		return nil
	default:
		return errors.New("invalid opinion kind")
	}
}
