/*
miningPoolCli – open-source tonuniverse mining pool client

Copyright (C) 2021 Alexander Gapak
Copyright (C) 2021 Kirill Glushakov
Copyright (C) 2021 Roman Klimov

This file is part of miningPoolCli.

miningPoolCli is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

miningPoolCli is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with miningPoolCli.  If not, see <https://www.gnu.org/licenses/>.
*/

package helpers

import (
	"miningPoolCli/utils/miniLogger"
	"os/exec"
)

func ExecuteSimpleCommand(name string, arg ...string) []byte {
	stdout, err := exec.Command(name, arg...).Output()
	if err != nil {
		miniLogger.LogFatal("Error while executing sh: " + "\"" + name + "\"" + "; " + err.Error())
	}
	return stdout
}
