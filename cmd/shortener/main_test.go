package main

import (
	"bytes"
	"testing"
)

func TestPrintBuildInfoUsesNAForEmptyValues(t *testing.T) {
	withBuildInfo("", "", "", func() {
		var buf bytes.Buffer

		printBuildInfo(&buf)

		want := "Build version: N/A\n" +
			"Build date: N/A\n" +
			"Build commit: N/A\n"
		if got := buf.String(); got != want {
			t.Fatalf("unexpected build info:\nwant:\n%s\ngot:\n%s", want, got)
		}
	})
}

func TestPrintBuildInfoUsesConfiguredValues(t *testing.T) {
	withBuildInfo("v1.2.3", "2026-07-02", "abc123", func() {
		var buf bytes.Buffer

		printBuildInfo(&buf)

		want := "Build version: v1.2.3\n" +
			"Build date: 2026-07-02\n" +
			"Build commit: abc123\n"
		if got := buf.String(); got != want {
			t.Fatalf("unexpected build info:\nwant:\n%s\ngot:\n%s", want, got)
		}
	})
}

func withBuildInfo(version, date, commit string, test func()) {
	oldVersion := buildVersion
	oldDate := buildDate
	oldCommit := buildCommit
	defer func() {
		buildVersion = oldVersion
		buildDate = oldDate
		buildCommit = oldCommit
	}()

	buildVersion = version
	buildDate = date
	buildCommit = commit
	test()
}
