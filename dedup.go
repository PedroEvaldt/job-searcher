package main

import (
	"regexp"
	"strings"

	"github.com/PedroEvaldt/job-searcher/internal/sources"
)

// nonKeyChars matches every character that is dropped from a dedupe key: anything
// that is not a lowercase letter or a digit. Same rule as db/queries/duplicates.sql.
var nonKeyChars = regexp.MustCompile("[^a-z0-9]")

// normalize turns a job's company and title into the two parts of its dedupe key,
// so "Full-stack" and "Full stack" compare equal. Text with no a-z or 0-9 at all
// (a title in Chinese, for example) becomes "".
func normalize(company, title string) (string, string) {
	normCompany := nonKeyChars.ReplaceAllString(strings.ToLower(company), "")
	normTitle := nonKeyChars.ReplaceAllString(strings.ToLower(title), "")
	return normCompany, normTitle
}

// isDuplicate reports whether job1, from source sID1, and job2, from source sID2,
// are the same real-world job: same company and title once normalized, and either
// different sources or an identical description. Within one source a different
// description may be a different job (Bjak posts the same title as hybrid and as
// remote). A key with an empty part is never a duplicate.
func isDuplicate(job1, job2 sources.Job, sID1, sID2 int) bool {
	normCompany1, normTitle1 := normalize(job1.Company, job1.Title)
	normCompany2, normTitle2 := normalize(job2.Company, job2.Title)

	if normTitle1 == "" || normTitle2 == "" || normCompany1 == "" || normCompany2 == "" {
		return false
	}
	if normCompany1 != normCompany2 || normTitle1 != normTitle2 {
		return false
	}

	if sID1 != sID2 {
		return true
	}
	return job1.Description == job2.Description
}
