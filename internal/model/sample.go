package model

// SampleCollection returns an in-memory collection used until collections are
// loaded from disk.
func SampleCollection() *Collection {
	get := func(name, url, description string) Request {
		r := NewRequest()
		r.Name, r.URL, r.Description = name, url, description
		return r
	}
	withMethod := func(r Request, m Method) Request { r.Method = m; return r }
	withJSON := func(r Request, body string) Request {
		r.Body = Body{Type: BodyRaw, Raw: body, ContentType: "application/json"}
		r.Headers = append(r.Headers, KeyValue{Name: "Content-Type", Value: "application/json", Enabled: true})
		return r
	}

	listUsers := get("List users", "${BASE_URL}/users", "Fetch a page of users.\n\nSupports `page` and `per_page` query parameters.")
	listUsers.Query = []KeyValue{{Name: "page", Value: "1", Enabled: true}, {Name: "per_page", Value: "20", Enabled: true}}
	listUsers.Headers = []KeyValue{{Name: "Accept", Value: "application/json", Enabled: true}}
	listUsers.File = "users/list-users.posting.yaml"

	getUser := get("Get user", "${BASE_URL}/users/:id", "Fetch a single user by ID.")
	getUser.PathParams = []KeyValue{{Name: "id", Value: "42", Enabled: true}}
	getUser.File = "users/get-user.posting.yaml"

	createUser := withJSON(withMethod(get("Create user", "${BASE_URL}/users", "Create a new user account."), MethodPost),
		"{\n  \"name\": \"Ada Lovelace\",\n  \"email\": \"ada@example.com\",\n  \"role\": \"admin\"\n}")
	createUser.Auth = Auth{Type: AuthBearer, Token: "${API_TOKEN}"}
	createUser.File = "users/create-user.posting.yaml"

	updateUser := withJSON(withMethod(get("Update user", "${BASE_URL}/users/:id", ""), MethodPatch),
		"{\n  \"role\": \"editor\"\n}")
	updateUser.PathParams = []KeyValue{{Name: "id", Value: "42", Enabled: true}}
	updateUser.File = "users/update-user.posting.yaml"

	deleteUser := withMethod(get("Delete user", "${BASE_URL}/users/:id", "Permanently delete a user."), MethodDelete)
	deleteUser.PathParams = []KeyValue{{Name: "id", Value: "42", Enabled: true}}
	deleteUser.File = "users/delete-user.posting.yaml"

	login := withMethod(get("Login", "${BASE_URL}/auth/login", "Exchange credentials for a session token."), MethodPost)
	login.Body = Body{Type: BodyForm, ContentType: "application/x-www-form-urlencoded", Form: []KeyValue{
		{Name: "username", Value: "ada", Enabled: true},
		{Name: "password", Value: "${PASSWORD}", Enabled: true},
	}}
	login.File = "auth/login.posting.yaml"

	me := get("Current user", "${BASE_URL}/auth/me", "")
	me.Auth = Auth{Type: AuthBasic, Username: "ada", Password: "${PASSWORD}"}
	me.File = "auth/me.posting.yaml"

	health := get("Health check", "https://httpbin.org/get", "Quick sanity check against httpbin.")
	health.File = "health.posting.yaml"
	options := withMethod(get("CORS preflight", "https://httpbin.org/anything", ""), MethodOptions)
	options.File = "cors-preflight.posting.yaml"

	root := &Collection{
		Name:     "sample-api",
		Requests: []Request{health, options},
		Children: []*Collection{
			{Name: "users", Path: "users", Requests: []Request{listUsers, getUser, createUser, updateUser, deleteUser}},
			{Name: "auth", Path: "auth", Requests: []Request{login, me}},
		},
	}
	root.Sort()
	return root
}

// SampleEnvironments returns environments used until env files are loaded.
func SampleEnvironments() []Environment {
	return []Environment{
		{Name: "local", Files: []string{"local.env"}, Variables: []Variable{
			{Name: "BASE_URL", Value: "http://localhost:8000", Source: "local.env"},
			{Name: "API_TOKEN", Value: "dev-token-123", Source: "local.env"},
			{Name: "PASSWORD", Value: "hunter2", Source: "local.env"},
		}},
		{Name: "staging", Files: []string{"staging.env"}, Variables: []Variable{
			{Name: "BASE_URL", Value: "https://staging.example.com/api", Source: "staging.env"},
			{Name: "API_TOKEN", Value: "stg-8f2a1c", Source: "staging.env"},
			{Name: "PASSWORD", Value: "correct-horse", Source: "staging.env"},
		}},
	}
}
