package tool

// This file holds the Validate implementation of every tool argument struct.
// registerTool runs arguments through the validated interface before a handler
// is invoked, so no caller supplied identifier reaches the Drone API client
// unchecked. Adding a tool therefore means adding a Validate method: the
// registration helper does not compile without one.

// validateOwnerRepo validates the repository identifier pair shared by most
// tools.
func validateOwnerRepo(owner, repo string) error {
	return firstErr(ValidateSegment("owner", owner), ValidateSegment("repo", repo))
}

// validateNumber is the error-only form of ValidateNumber.
func validateNumber(field string, value float64) error {
	_, err := ValidateNumber(field, value)
	return err
}

// --- Repository tools ---

func (ListReposArgs) Validate() error { return nil }

func (a GetRepoArgs) Validate() error { return validateOwnerRepo(a.Owner, a.Repo) }

func (a EnableRepoArgs) Validate() error { return validateOwnerRepo(a.Owner, a.Repo) }

func (a DisableRepoArgs) Validate() error { return validateOwnerRepo(a.Owner, a.Repo) }

func (a RepairRepoArgs) Validate() error { return validateOwnerRepo(a.Owner, a.Repo) }

func (a ChownRepoArgs) Validate() error { return validateOwnerRepo(a.Owner, a.Repo) }

func (SyncReposArgs) Validate() error { return nil }

func (ListIncompleteArgs) Validate() error { return nil }

// --- Build tools ---

func (a ListBuildsArgs) Validate() error { return validateOwnerRepo(a.Owner, a.Repo) }

func (a GetBuildArgs) Validate() error {
	return firstErr(validateOwnerRepo(a.Owner, a.Repo), validateNumber("build", a.Build))
}

func (a GetBuildLastArgs) Validate() error {
	return firstErr(validateOwnerRepo(a.Owner, a.Repo), ValidateBranch("branch", a.Branch))
}

func (a BuildLogsArgs) Validate() error {
	return firstErr(
		validateOwnerRepo(a.Owner, a.Repo),
		validateNumber("build", a.Build),
		validateNumber("stage", a.Stage),
		validateNumber("step", a.Step),
	)
}

func (a RestartBuildArgs) Validate() error {
	return firstErr(
		validateOwnerRepo(a.Owner, a.Repo),
		validateNumber("build", a.Build),
		ValidateParams("params", a.Params),
	)
}

func (a CancelBuildArgs) Validate() error {
	return firstErr(validateOwnerRepo(a.Owner, a.Repo), validateNumber("build", a.Build))
}

func (a PromoteBuildArgs) Validate() error {
	return firstErr(
		validateOwnerRepo(a.Owner, a.Repo),
		validateNumber("build", a.Build),
		ValidateSegment("target", a.Target),
		ValidateParams("params", a.Params),
	)
}

func (a RollbackBuildArgs) Validate() error {
	return firstErr(
		validateOwnerRepo(a.Owner, a.Repo),
		validateNumber("build", a.Build),
		ValidateSegment("target", a.Target),
		ValidateParams("params", a.Params),
	)
}

func (a ApproveBuildArgs) Validate() error {
	return firstErr(
		validateOwnerRepo(a.Owner, a.Repo),
		validateNumber("build", a.Build),
		validateNumber("stage", a.Stage),
	)
}

func (a DeclineBuildArgs) Validate() error {
	return firstErr(
		validateOwnerRepo(a.Owner, a.Repo),
		validateNumber("build", a.Build),
		validateNumber("stage", a.Stage),
	)
}

func (a CreateBuildArgs) Validate() error {
	return firstErr(
		validateOwnerRepo(a.Owner, a.Repo),
		ValidateCommit("commit", a.Commit),
		ValidateBranch("branch", a.Branch),
		ValidateParams("params", a.Params),
	)
}

// --- Cron tools ---

func (a ListCronsArgs) Validate() error { return validateOwnerRepo(a.Owner, a.Repo) }

func (a GetCronArgs) Validate() error {
	return firstErr(validateOwnerRepo(a.Owner, a.Repo), ValidateSegment("cron", a.Cron))
}

func (a CreateCronArgs) Validate() error {
	return firstErr(
		validateOwnerRepo(a.Owner, a.Repo),
		ValidateSegment("name", a.Name),
		ValidateText("expr", a.Expr, maxExprLength),
		ValidateBranch("branch", a.Branch),
	)
}

func (a DeleteCronArgs) Validate() error {
	return firstErr(validateOwnerRepo(a.Owner, a.Repo), ValidateSegment("cron", a.Cron))
}

func (a ExecuteCronArgs) Validate() error {
	return firstErr(validateOwnerRepo(a.Owner, a.Repo), ValidateSegment("cron", a.Cron))
}

// --- Repository secret tools ---

func (a ListSecretsArgs) Validate() error { return validateOwnerRepo(a.Owner, a.Repo) }

func (a GetSecretArgs) Validate() error {
	return firstErr(validateOwnerRepo(a.Owner, a.Repo), ValidateSegment("name", a.Name))
}

func (a CreateSecretArgs) Validate() error {
	return firstErr(
		validateOwnerRepo(a.Owner, a.Repo),
		ValidateSegment("name", a.Name),
		ValidateText("value", a.Value, maxSecretValueLength),
	)
}

func (a UpdateSecretArgs) Validate() error {
	return firstErr(
		validateOwnerRepo(a.Owner, a.Repo),
		ValidateSegment("name", a.Name),
		ValidateText("value", a.Value, maxSecretValueLength),
	)
}

func (a DeleteSecretArgs) Validate() error {
	return firstErr(validateOwnerRepo(a.Owner, a.Repo), ValidateSegment("name", a.Name))
}

// --- Organization secret tools ---

func (a ListOrgSecretsArgs) Validate() error {
	return ValidateSegment("namespace", a.Namespace)
}

func (a GetOrgSecretArgs) Validate() error {
	return firstErr(ValidateSegment("namespace", a.Namespace), ValidateSegment("name", a.Name))
}

func (a CreateOrgSecretArgs) Validate() error {
	return firstErr(
		ValidateSegment("namespace", a.Namespace),
		ValidateSegment("name", a.Name),
		ValidateText("value", a.Value, maxSecretValueLength),
	)
}

func (a UpdateOrgSecretArgs) Validate() error {
	return firstErr(
		ValidateSegment("namespace", a.Namespace),
		ValidateSegment("name", a.Name),
		ValidateText("value", a.Value, maxSecretValueLength),
	)
}

func (a DeleteOrgSecretArgs) Validate() error {
	return firstErr(ValidateSegment("namespace", a.Namespace), ValidateSegment("name", a.Name))
}

// --- User tools ---

func (GetSelfArgs) Validate() error { return nil }

func (ListUsersArgs) Validate() error { return nil }

func (a GetUserArgs) Validate() error { return ValidateSegment("login", a.Login) }

func (a CreateUserArgs) Validate() error {
	return firstErr(
		ValidateSegment("login", a.Login),
		ValidateOptionalText("email", a.Email, maxEmailLength),
		ValidateOptionalText("token", a.Token, maxTokenLength),
	)
}

func (a UpdateUserArgs) Validate() error {
	return firstErr(
		ValidateSegment("login", a.Login),
		ValidateOptionalText("email", a.Email, maxEmailLength),
	)
}

func (a DeleteUserArgs) Validate() error { return ValidateSegment("login", a.Login) }

// --- Template tools ---

func (a ListTemplatesArgs) Validate() error {
	return ValidateOptionalSegment("namespace", a.Namespace)
}

func (a GetTemplateArgs) Validate() error {
	return firstErr(ValidateSegment("namespace", a.Namespace), ValidateSegment("name", a.Name))
}

func (a CreateTemplateArgs) Validate() error {
	return firstErr(
		ValidateSegment("namespace", a.Namespace),
		ValidateSegment("name", a.Name),
		ValidateText("data", a.Data, maxTemplateLength),
	)
}

func (a UpdateTemplateArgs) Validate() error {
	return firstErr(
		ValidateSegment("namespace", a.Namespace),
		ValidateSegment("name", a.Name),
		ValidateText("data", a.Data, maxTemplateLength),
	)
}

func (a DeleteTemplateArgs) Validate() error {
	return firstErr(ValidateSegment("namespace", a.Namespace), ValidateSegment("name", a.Name))
}
