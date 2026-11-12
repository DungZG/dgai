/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Opus 4.8 bills output at roughly 1.67x input upstream, unlike the earlier
// Opus 4 models that the family default of 5 was written for. A configured
// ratio must therefore win; the family default stays as the fallback only.
func TestOpus48CompletionRatioHonoursConfig(t *testing.T) {
	name := string([]byte{
		0x63, 0x6c, 0x61, 0x75, 0x64, 0x65, 0x2d, // claude-
		0x6f, 0x70, 0x75, 0x73, 0x2d, 0x34, 0x2d, 0x38, // opus-4-8
	})

	configured := 1.6667
	info := ResolveCompletionRatio(name, &configured)
	require.Equal(t, 1.6667, info.Ratio)
	require.False(t, info.Locked, "the administrator's ratio must not be locked out")

	fallback := ResolveCompletionRatio(name, nil)
	require.Equal(t, 5.0, fallback.Ratio, "the family default still applies when nothing is configured")
}
