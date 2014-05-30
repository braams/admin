package admin

import (
	"bytes"
	"fmt"
	"github.com/gorilla/mux"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

func (a *Admin) render(rw http.ResponseWriter, tmpl string, ctx map[string]interface{}) {
	ctx["title"] = a.Title
	ctx["path"] = a.Path
	if _, ok := ctx["anonymous"]; !ok {
		fmt.Println(ctx["anonymous"])
		ctx["anonymous"] = false
	}

	err := templates.ExecuteTemplate(rw, tmpl, ctx)
	if err != nil {
		fmt.Println(err)
	}
}

// handlerWrapper is used to redirect to index / log in page.
func (a *Admin) handlerWrapper(h http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		if !a.isLoggedIn(req) && req.URL.Path != a.Path+"/" {
			http.Redirect(rw, req, a.Path, 302)
			return
		}
		h.ServeHTTP(rw, req)
	}
}

func (a *Admin) handleIndex(rw http.ResponseWriter, req *http.Request) {
	if !a.isLoggedIn(req) {
		if req.Method == "POST" {
			req.ParseForm()
			ok := a.logIn(rw, req.Form.Get("username"), req.Form.Get("password"))
			if ok {
				http.Redirect(rw, req, a.Path, 302)
			}
		}
		a.render(rw, "login.html", map[string]interface{}{
			"anonymous": true,
		})
		return
	}
	a.render(rw, "index.html", map[string]interface{}{
		"groups": a.modelGroups,
	})
}

func (a *Admin) handleList(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	slug := vars["slug"]

	model, ok := a.models[slug]
	if !ok {
		http.NotFound(rw, req)
		return
	}

	results, err := a.queryModel(model)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(results)

	a.render(rw, "list.html", map[string]interface{}{
		"name":    model.Name,
		"slug":    slug,
		"columns": model.listColumns(),
		"results": results,
	})
}

func (a *Admin) handleEdit(rw http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		a.handleSave(rw, req)
		return
	}
	vars := mux.Vars(req)
	slug := vars["slug"]

	model, ok := a.models[slug]
	if !ok {
		http.NotFound(rw, req)
		return
	}

	// Get model data
	var data []interface{}
	var id int
	if idStr, ok := vars["id"]; ok {
		id64, err := strconv.ParseInt(idStr, 10, 64)
		id = int(id64)
		if err != nil {
			http.NotFound(rw, req)
			return
		}
		data, err = a.querySingleModel(model, id)
		if err != nil {
			fmt.Println(err)
			http.NotFound(rw, req)
			return
		}
		data = data[1:]
		fmt.Println(data)
	}

	// Render form and template
	var buf bytes.Buffer
	model.renderForm(&buf, data)

	a.render(rw, "edit.html", map[string]interface{}{
		"id":   id,
		"name": model.Name,
		"form": template.HTML(buf.String()),
	})
}

func (a *Admin) handleSave(rw http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		fmt.Println(err)
		return
	}

	vars := mux.Vars(req)
	slug := vars["slug"]
	model, ok := a.models[slug]
	if !ok {
		http.NotFound(rw, req)
		return
	}

	id := 0
	if idStr, ok := vars["id"]; ok {
		id, err = parseInt(idStr)
		if err != nil {
			return
		}
	}

	numFields := len(model.fieldNames())

	// Create query
	valMarks := strings.Repeat("?, ", numFields)
	valMarks = valMarks[0 : len(valMarks)-2]

	var q string
	if id != 0 {
		keys := make([]string, numFields)
		for i := 0; i < numFields; i++ {
			keys[i] = fmt.Sprintf("%v = ?", model.tableColumns()[i])
		}
		q = fmt.Sprintf("UPDATE %v SET %v WHERE id = %v", model.tableName, strings.Join(keys, ", "), id)
	} else {
		q = fmt.Sprintf("INSERT INTO %v(%v) VALUES(%v)", model.tableName, strings.Join(model.tableColumns(), ", "), valMarks)
	}

	// Get data from POST and fill a slice
	data := make([]interface{}, numFields)
	for i := 0; i < numFields; i++ {
		fieldName := model.fieldByName(model.fieldNames()[i])
		val, err := fieldName.field.Validate(req.Form.Get(model.fieldNames()[i]))
		if err != nil {
			fmt.Println(err)
			return
		}
		data[i] = val
	}

	_, err = a.db.Exec(q, data...)
	if err != nil {
		fmt.Println(err)
		return
	}

	if req.Form.Get("done") == "true" {
		http.Redirect(rw, req, a.modelURL(slug, ""), 302)
	} else {
		http.Redirect(rw, req, a.modelURL(slug, fmt.Sprintf("/edit/%v", id)), 302)
	}
}
