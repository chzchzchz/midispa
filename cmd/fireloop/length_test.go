package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Shortening a pattern leaves the step cursor past its new end, and the rows are drawn
// from it, so the press that shortens it has to pull the cursor back inside. The redraw
// that replaced the navigation call does not do this on its own, which is why the length
// code says so.
func TestShorteningAPatternPullsTheStepCursorBack(t *testing.T) {
	bank, _ := chromaBank(t)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.MoveStepCursor(maxPatternSteps-1))
	require.Equal(t, maxPatternSteps-1, bank.StepCursor(), "the cursor did not reach the last step")

	require.NoError(t, bank.ToggleLengthMode())
	require.True(t, bank.editingLength, "the length mode did not open")
	for bank.CurrentPattern().LengthSteps() > 4 {
		require.NoError(t, bank.AdjustLength(-1))
	}

	require.Lessf(t, bank.StepCursor(), bank.CurrentPattern().LengthSteps(),
		"the step cursor is past the end of the pattern it is editing")
}
