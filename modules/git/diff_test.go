// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const exampleDiff = `diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1,3 +1,6 @@
 # gitea-github-migrator
+
+ Build Status
- Latest Release
 Docker Pulls
+ cut off
+ cut off`

const breakingDiff = `diff --git a/aaa.sql b/aaa.sql
index d8e4c92..19dc8ad 100644
--- a/aaa.sql
+++ b/aaa.sql
@@ -1,9 +1,10 @@
 --some comment
--- some comment 5
+--some comment 2
+-- some comment 3
 create or replace procedure test(p1 varchar2)
 is
 begin
---new comment
 dbms_output.put_line(p1);
+--some other comment
 end;
 /
`

var issue17875Diff = `diff --git a/Geschäftsordnung.md b/Geschäftsordnung.md
index d46c152..a7d2d55 100644
--- a/Geschäftsordnung.md
+++ b/Geschäftsordnung.md
@@ -1,5 +1,5 @@
 ---
-date: "23.01.2021"
+date: "30.11.2021"
 ...
 ` + `
 # Geschäftsordnung
@@ -16,4 +16,22 @@ Diese Geschäftsordnung regelt alle Prozesse des Vereins, solange diese nicht du
 ` + `
 ## § 3 Datenschutzverantwortlichkeit
 ` + `
-1. Der Verein bestellt eine datenschutzverantwortliche Person mit den Aufgaben nach Artikel 39 DSGVO.
\ No newline at end of file
+1. Der Verein bestellt eine datenschutzverantwortliche Person mit den Aufgaben nach Artikel 39 DSGVO.
+
+## §4 Umgang mit der SARS-Cov-2-Pandemie
+
+1. Der Vorstand hat die Befugnis, in Rücksprache mit den Vereinsmitgliedern, verschiedene Hygienemaßnahmen für Präsenzveranstaltungen zu beschließen.
+
+2. Die Einführung, Änderung und Abschaffung dieser Maßnahmen sind nur zum Zweck der Eindämmung der SARS-Cov-2-Pandemie zulässig.
+
+3. Die Einführung, Änderung und Abschaffung von Maßnahmen nach Abs. 2 bedarf einer wissenschaftlichen Grundlage.
+
+4. Die Maßnahmen nach Abs. 2 setzen sich aus den folgenden Bausteinen inklusive einer ihrer Ausprägungen zusammen.
+
+	1. Maskenpflicht: Keine; Maskenpflicht, außer am Platz, oder wo Abstände nicht eingehalten werden können; Maskenpflicht, wenn Abstände nicht eingehalten werden können;  Maskenpflicht
+
+	2. Geimpft-, Genesen- oder Testnachweis: Kein Nachweis notwendig; Nachweis, dass Person geimpft, genesen oder tagesaktuell getestet ist (3G); Nachweis, dass Person geimpft oder genesen ist (2G); Nachweis, dass Person geimpft bzw. genesen und tagesaktuell getestet ist (2G+)
+
+	3. Online-Veranstaltung: Keine, parallele Online-Veranstaltung, ausschließlich Online-Veranstaltung
+
+5. Bei Präsenzveranstungen gelten außerdem die Hygienevorschriften des Veranstaltungsorts. Bei Regelkollision greift die restriktivere Regel.
\ No newline at end of file`

func TestCutDiffAroundLineIssue17875(t *testing.T) {
	result, err := CutDiffAroundLine(strings.NewReader(issue17875Diff), 23, false, 3)
	require.NoError(t, err)
	expected := `diff --git a/Geschäftsordnung.md b/Geschäftsordnung.md
--- a/Geschäftsordnung.md
+++ b/Geschäftsordnung.md
@@ -20,0 +21,3 @@
+## §4 Umgang mit der SARS-Cov-2-Pandemie
+
+1. Der Vorstand hat die Befugnis, in Rücksprache mit den Vereinsmitgliedern, verschiedene Hygienemaßnahmen für Präsenzveranstaltungen zu beschließen.`
	assert.Equal(t, expected, result)
}

func TestCutDiffAroundLine(t *testing.T) {
	result, err := CutDiffAroundLine(strings.NewReader(exampleDiff), 4, false, 3)
	require.NoError(t, err)
	resultByLine := strings.Split(result, "\n")
	assert.Len(t, resultByLine, 7)
	// Check if headers got transferred
	assert.Equal(t, "diff --git a/README.md b/README.md", resultByLine[0])
	assert.Equal(t, "--- a/README.md", resultByLine[1])
	assert.Equal(t, "+++ b/README.md", resultByLine[2])
	// Check if hunk header is calculated correctly
	assert.Equal(t, "@@ -2,2 +3,2 @@", resultByLine[3])
	// Check if line got transferred
	assert.Equal(t, "+ Build Status", resultByLine[4])

	// Must be same result as before since old line 3 == new line 5
	newResult, err := CutDiffAroundLine(strings.NewReader(exampleDiff), 3, true, 3)
	require.NoError(t, err)
	assert.Equal(t, result, newResult, "Must be same result as before since old line 3 == new line 5")

	newResult, err = CutDiffAroundLine(strings.NewReader(exampleDiff), 6, false, 300)
	require.NoError(t, err)
	assert.Equal(t, exampleDiff, newResult)

	emptyResult, err := CutDiffAroundLine(strings.NewReader(exampleDiff), 6, false, 0)
	require.NoError(t, err)
	assert.Empty(t, emptyResult)

	// Line is out of scope
	emptyResult, err = CutDiffAroundLine(strings.NewReader(exampleDiff), 434, false, 0)
	require.NoError(t, err)
	assert.Empty(t, emptyResult)

	// Handle minus diffs properly
	minusDiff, err := CutDiffAroundLine(strings.NewReader(breakingDiff), 2, false, 4)
	require.NoError(t, err)

	expected := `diff --git a/aaa.sql b/aaa.sql
--- a/aaa.sql
+++ b/aaa.sql
@@ -1,9 +1,10 @@
 --some comment
--- some comment 5
+--some comment 2`
	assert.Equal(t, expected, minusDiff)

	// Handle minus diffs properly
	minusDiff, err = CutDiffAroundLine(strings.NewReader(breakingDiff), 3, false, 4)
	require.NoError(t, err)

	expected = `diff --git a/aaa.sql b/aaa.sql
--- a/aaa.sql
+++ b/aaa.sql
@@ -1,9 +1,10 @@
 --some comment
--- some comment 5
+--some comment 2
+-- some comment 3`

	assert.Equal(t, expected, minusDiff)
}

func BenchmarkCutDiffAroundLine(b *testing.B) {
	for n := 0; n < b.N; n++ {
		CutDiffAroundLine(strings.NewReader(exampleDiff), 3, true, 3)
	}
}

func TestParseDiffHunkString(t *testing.T) {
	leftLine, leftHunk, rightLine, rightHunk := ParseDiffHunkString("@@ -19,3 +19,5 @@ AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER")
	assert.Equal(t, 19, leftLine)
	assert.Equal(t, 3, leftHunk)
	assert.Equal(t, 19, rightLine)
	assert.Equal(t, 5, rightHunk)
}

func TestFindAdjustedLineNumber(t *testing.T) {
	commentCutDiff := `diff --git a/file1.md b/file1.md
--- a/file1.md
+++ b/file1.md
@@ -47,7 +47,6 @@ Line 46
 Line 47
 Line 48
 Line 49
-Line 50`

	t.Run("no additional changes", func(t *testing.T) {
		diff := `diff --git a/file1.md b/file1.md
index 2d203fb..b21df3f 100644
--- a/file1.md
+++ b/file1.md
@@ -47,7 +47,6 @@ Line 46
 Line 47
 Line 48
 Line 49
-Line 50
 Line 51
 Line 52
 Line 53`
		lineNumber, err := FindAdjustedLineNumber(commentCutDiff, 50, strings.NewReader(diff))
		require.NoError(t, err)
		assert.Equal(t, LinePlacement{Left: 50, Right: 50}, lineNumber)
	})

	t.Run("removed lines before location", func(t *testing.T) {
		diff := `diff --git a/file1.md b/file1.md
index 2d203fb..c85b903 100644
--- a/file1.md
+++ b/file1.md
@@ -1,13 +1,3 @@
-Line 1
-Line 2
-Line 3
-Line 4
-Line 5
-Line 6
-Line 7
-Line 8
-Line 9
-Line 10
 Line 11
 Line 12
 Line 13
@@ -47,7 +37,6 @@ Line 46
 Line 47
 Line 48
 Line 49
-Line 50
 Line 51
 Line 52
 Line 53`
		lineNumber, err := FindAdjustedLineNumber(commentCutDiff, 50, strings.NewReader(diff))
		require.NoError(t, err)
		assert.Equal(t, LinePlacement{Left: 50, Right: 40}, lineNumber)
	})

	t.Run("added lines before location", func(t *testing.T) {
		diff := `diff --git a/file1.md b/file1.md
index 2d203fb..24b1aa6 100644
--- a/file1.md
+++ b/file1.md
@@ -8,6 +8,11 @@ Line 7
 Line 8
 Line 9
 Line 10
+Line 10.1
+Line 10.2
+Line 10.3
+Line 10.4
+Line 10.5
 Line 11
 Line 12
 Line 13
@@ -47,7 +52,6 @@ Line 46
 Line 47
 Line 48
 Line 49
-Line 50
 Line 51
 Line 52
 Line 53`
		lineNumber, err := FindAdjustedLineNumber(commentCutDiff, 50, strings.NewReader(diff))
		require.NoError(t, err)
		assert.Equal(t, LinePlacement{Left: 50, Right: 55}, lineNumber)
	})

	t.Run("added and removed in lines before location", func(t *testing.T) {
		diff := `diff --git a/file1.md b/file1.md
index 2d203fb..d0cb63f 100644
--- a/file1.md
+++ b/file1.md
@@ -5,9 +5,11 @@ Line 4
 Line 5
 Line 6
 Line 7
-Line 8
-Line 9
-Line 10
+Line 10.1
+Line 10.2
+Line 10.3
+Line 10.4
+Line 10.5
 Line 11
 Line 12
 Line 13
@@ -47,7 +49,6 @@ Line 46
 Line 47
 Line 48
 Line 49
-Line 50
 Line 51
 Line 52
 Line 53`
		lineNumber, err := FindAdjustedLineNumber(commentCutDiff, 50, strings.NewReader(diff))
		require.NoError(t, err)
		assert.Equal(t, LinePlacement{Left: 50, Right: 52}, lineNumber)
	})

	t.Run("changes above in the same hunk", func(t *testing.T) {
		diff := `diff --git a/file1.md b/file1.md
index 2d203fb..f35a466 100644
--- a/file1.md
+++ b/file1.md
@@ -42,12 +42,6 @@ Line 41
 Line 42
 Line 43
 Line 44
-Line 45
-Line 46
-Line 47
-Line 48
-Line 49
-Line 50
 Line 51
 Line 52
 Line 53`
		lineNumber, err := FindAdjustedLineNumber(commentCutDiff, 50, strings.NewReader(diff))
		require.NoError(t, err)
		assert.Equal(t, LinePlacement{Left: 50, Right: 45}, lineNumber)
	})

	t.Run("first line in diff", func(t *testing.T) {
		commentCutDiff := `diff --git a/file1.md b/file1.md
--- a/file1.md
+++ b/file1.md
@@ -1,4 +1,3 @@
-Line 1`
		diff := `diff --git a/file1.md b/file1.md
index 2d203fb..a490028 100644
--- a/file1.md
+++ b/file1.md
@@ -1,4 +1,3 @@
-Line 1
 Line 2
 Line 3
 Line 4`
		lineNumber, err := FindAdjustedLineNumber(commentCutDiff, 1, strings.NewReader(diff))
		require.NoError(t, err)
		assert.Equal(t, LinePlacement{Left: 1, Right: 1}, lineNumber)
	})

	t.Run("adjusted line not found", func(t *testing.T) {
		// "Line 50" is present here but it's no longer "-Line 50", so it should not be identified as present
		diff := `diff --git a/file1.md b/file1.md
index 2d203fb..09dd95a 100644
--- a/file1.md
+++ b/file1.md
@@ -42,10 +42,6 @@ Line 41
 Line 42
 Line 43
 Line 44
-Line 45
-Line 46
-Line 47
-Line 48
 Line 49
 Line 50
 Line 51`
		_, err := FindAdjustedLineNumber(commentCutDiff, 50, strings.NewReader(diff))
		require.ErrorIs(t, err, ErrLineNotFound)
	})

	t.Run("adjusted line hunk not present - not changed anymore", func(t *testing.T) {
		diff := `diff --git a/file1.md b/file1.md
index 2d203fb..d0cb63f 100644
--- a/file1.md
+++ b/file1.md
@@ -5,9 +5,11 @@ Line 4
 Line 5
 Line 6
 Line 7
-Line 8
-Line 9
-Line 10
+Line 10.1
+Line 10.2
+Line 10.3
+Line 10.4
+Line 10.5
 Line 11
 Line 12
 Line 13`
		_, err := FindAdjustedLineNumber(commentCutDiff, 50, strings.NewReader(diff))
		require.ErrorIs(t, err, ErrLineNotFound)
	})
}
