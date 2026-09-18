package tool

import (
	"math"
	"strings"
	"testing"
)

func TestValidateSegment(t *testing.T) {
	valid := []string{"repo", "my-repo.v2", "a_1", "9x", "Drone"}
	for _, value := range valid {
		if err := ValidateSegment("owner", value); err != nil {
			t.Fatalf("ValidateSegment(%q) = %v, want nil", value, err)
		}
	}

	invalid := map[string]string{
		"empty":                 "",
		"dot":                   ".",
		"dot dot":               "..",
		"parent traversal":      "../users",
		"slash":                 "a/b",
		"backslash":             `a\b`,
		"query":                 "a?b",
		"fragment":              "a#b",
		"percent":               "a%2fb",
		"ampersand":             "a&b",
		"equals":                "a=b",
		"space":                 "a b",
		"leading hyphen":        "-flag",
		"newline":               "a\nb",
		"carriage return":       "a\rb",
		"tab":                   "a\tb",
		"non ascii":             "répo",
		"too long":              strings.Repeat("a", maxSegmentLength+1),
		"hidden parent segment": "a..b/..",
	}
	for name, value := range invalid {
		if err := ValidateSegment("owner", value); err == nil {
			t.Fatalf("ValidateSegment(%q) accepted the %s case", value, name)
		}
	}
}

func TestValidateOptionalSegment(t *testing.T) {
	if err := ValidateOptionalSegment("namespace", ""); err != nil {
		t.Fatalf("ValidateOptionalSegment(\"\") = %v, want nil", err)
	}
	if err := ValidateOptionalSegment("namespace", "../x"); err == nil {
		t.Fatal("ValidateOptionalSegment accepted a traversing value")
	}
}

func TestValidateBranch(t *testing.T) {
	valid := []string{"", "main", "feature/x", "release-1.2", "a/b/c", "user/fix.1"}
	for _, value := range valid {
		if err := ValidateBranch("branch", value); err != nil {
			t.Fatalf("ValidateBranch(%q) = %v, want nil", value, err)
		}
	}

	invalid := []string{
		"../x", "x/../y", "x//y", "/x", "x/", "x&y", "x?y", "x y", "x#y", "x%2f",
		strings.Repeat("a", maxBranchLength+1),
	}
	for _, value := range invalid {
		if err := ValidateBranch("branch", value); err == nil {
			t.Fatalf("ValidateBranch(%q) accepted an invalid branch", value)
		}
	}
}

func TestValidateNumber(t *testing.T) {
	valid := map[float64]int{1: 1, 42: 42, maxNumber: maxNumber}
	for value, want := range valid {
		got, err := ValidateNumber("build", value)
		if err != nil {
			t.Fatalf("ValidateNumber(%v) = %v, want nil", value, err)
		}
		if got != want {
			t.Fatalf("ValidateNumber(%v) = %d, want %d", value, got, want)
		}
	}

	invalid := []float64{0, -1, 1.5, math.NaN(), math.Inf(1), maxNumber + 1}
	for _, value := range invalid {
		if _, err := ValidateNumber("build", value); err == nil {
			t.Fatalf("ValidateNumber(%v) accepted an invalid number", value)
		}
	}
}

func TestValidateParams(t *testing.T) {
	if err := ValidateParams("params", nil); err != nil {
		t.Fatalf("ValidateParams(nil) = %v, want nil", err)
	}
	if err := ValidateParams("params", map[string]string{"ENV": "production"}); err != nil {
		t.Fatalf("ValidateParams(valid) = %v, want nil", err)
	}

	tooMany := make(map[string]string, maxParams+1)
	for i := 0; i <= maxParams; i++ {
		tooMany[string(rune('A'+i%26))+string(rune('a'+i/26))] = "v"
	}
	if err := ValidateParams("params", tooMany); err == nil {
		t.Fatal("ValidateParams accepted too many entries")
	}
	if err := ValidateParams("params", map[string]string{"": "v"}); err == nil {
		t.Fatal("ValidateParams accepted an empty key")
	}
	if err := ValidateParams("params", map[string]string{"K": "a\nb"}); err == nil {
		t.Fatal("ValidateParams accepted a newline in a value")
	}
}

func TestValidateText(t *testing.T) {
	if err := ValidateText("value", "  ", maxSecretValueLength); err == nil {
		t.Fatal("ValidateText accepted a blank required value")
	}
	if err := ValidateText("value", "s3cret", maxSecretValueLength); err != nil {
		t.Fatalf("ValidateText(valid) = %v, want nil", err)
	}
	if err := ValidateText("value", strings.Repeat("x", maxSecretValueLength+1), maxSecretValueLength); err == nil {
		t.Fatal("ValidateText accepted an oversized value")
	}
	// Multi-line values are legitimate: template YAML and certificate style
	// secrets must keep working.
	if err := ValidateText("data", "kind: pipeline\nsteps:\n  - name: test\n", maxTemplateLength); err != nil {
		t.Fatalf("ValidateText rejected a multi-line value: %v", err)
	}

	if err := ValidateOptionalText("token", "", maxTokenLength); err != nil {
		t.Fatalf("ValidateOptionalText(\"\") = %v, want nil", err)
	}
}

func TestValidateCommit(t *testing.T) {
	for _, value := range []string{"", "deadbeef", "v1.2.3", "refs/heads/main", "feature/x"} {
		if err := ValidateCommit("commit", value); err != nil {
			t.Fatalf("ValidateCommit(%q) = %v, want nil", value, err)
		}
	}
	for _, value := range []string{"a b", "a&b", "a?b", strings.Repeat("a", maxCommitLength+1)} {
		if err := ValidateCommit("commit", value); err == nil {
			t.Fatalf("ValidateCommit(%q) accepted an invalid commit", value)
		}
	}
}

// TestArgsValidate covers the values that used to be forwarded to drone-go
// unchecked. Repository identifiers end up in the request path, so traversal
// and query injection must be rejected.
func TestArgsValidateRejectsInjection(t *testing.T) {
	cases := map[string]validatedArgs{
		"repo owner traversal":    GetRepoArgs{Owner: "../users", Repo: "drone"},
		"repo name traversal":     GetRepoArgs{Owner: "octocat", Repo: "../../api/users"},
		"repo query injection":    GetRepoArgs{Owner: "octocat", Repo: "drone?page=1"},
		"build branch injection":  GetBuildLastArgs{Owner: "octocat", Repo: "drone", Branch: "main&page=1"},
		"build number zero":       GetBuildArgs{Owner: "octocat", Repo: "drone", Build: 0},
		"build number float":      GetBuildArgs{Owner: "octocat", Repo: "drone", Build: 1.5},
		"log build number":        BuildLogsArgs{Owner: "octocat", Repo: "drone", Build: -3, Stage: 1, Step: 1},
		"secret name slash":       GetSecretArgs{Owner: "octocat", Repo: "drone", Name: "a/b"},
		"org namespace traversal": ListOrgSecretsArgs{Namespace: ".."},
		"user login query":        GetUserArgs{Login: "admin?x=1"},
		"cron name slash":         GetCronArgs{Owner: "octocat", Repo: "drone", Cron: "../x"},
		"cron expr empty":         CreateCronArgs{Owner: "octocat", Repo: "drone", Name: "nightly", Expr: ""},
		"template name traversal": GetTemplateArgs{Namespace: "octocat", Name: "../.."},
		"promote target slash":    PromoteBuildArgs{Owner: "octocat", Repo: "drone", Build: 1, Target: "prod/../x"},
		"empty owner":             GetRepoArgs{Owner: "", Repo: "drone"},
	}
	for name, args := range cases {
		if err := args.Validate(); err == nil {
			t.Fatalf("%s: Validate() = nil, want an error", name)
		}
	}
}

// TestArgsValidateAcceptsNormalValues guards against validation that is so
// strict it breaks legitimate usage.
func TestArgsValidateAcceptsNormalValues(t *testing.T) {
	cases := map[string]validatedArgs{
		"repo":             GetRepoArgs{Owner: "octocat", Repo: "hello-world.go"},
		"build":            GetBuildArgs{Owner: "octocat", Repo: "drone", Build: 42},
		"branch with path": GetBuildLastArgs{Owner: "octocat", Repo: "drone", Branch: "feature/new-thing"},
		"build logs":       BuildLogsArgs{Owner: "octocat", Repo: "drone", Build: 1, Stage: 2, Step: 3},
		"restart":          RestartBuildArgs{Owner: "octocat", Repo: "drone", Build: 1, Params: map[string]string{"ENV": "prod"}},
		"promote":          PromoteBuildArgs{Owner: "octocat", Repo: "drone", Build: 7, Target: "production"},
		"create build":     CreateBuildArgs{Owner: "octocat", Repo: "drone", Commit: "deadbeef", Branch: "main"},
		"cron":             CreateCronArgs{Owner: "octocat", Repo: "drone", Name: "nightly", Expr: "0 0 * * *", Branch: "main"},
		"secret":           CreateSecretArgs{Owner: "octocat", Repo: "drone", Name: "DOCKER_PASSWORD", Value: "s3cret"},
		"org secret":       CreateOrgSecretArgs{Namespace: "octocat", Name: "TOKEN", Value: "abc"},
		"user":             CreateUserArgs{Login: "octocat", Email: "octocat@example.com", Admin: false, Active: true},
		"template":         CreateTemplateArgs{Namespace: "octocat", Name: "kubernetes", Data: "kind: pipeline\n"},
		"list templates":   ListTemplatesArgs{Namespace: ""},
		"no args":          ListReposArgs{},
	}
	for name, args := range cases {
		if err := args.Validate(); err != nil {
			t.Fatalf("%s: Validate() = %v, want nil", name, err)
		}
	}
}

// validatedArgs lets the tables above hold any tool argument struct.
type validatedArgs interface {
	Validate() error
}
