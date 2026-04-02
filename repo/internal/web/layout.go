package web

import (
	"context"
	"fmt"
	"html"
	"io"
	"sort"
	"strings"

	"github.com/a-h/templ"

	"parkops/internal/auth"
)

type CurrentUser struct {
	DisplayName string
	Username    string
	Roles       []string
}

type navItem struct {
	Label string
	Path  string
	Roles []string
}

func AppLayout(user CurrentUser, pageTitle, currentPath, contentHTML string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_ = ctx
		fullName := strings.TrimSpace(user.DisplayName)
		if fullName == "" {
			fullName = user.Username
		}
		initials := initialsFor(fullName)
		if initials == "" {
			initials = "OP"
		}

		navHTML := buildNav(user.Roles, currentPath)
		roles := make([]string, 0, len(user.Roles))
		roles = append(roles, user.Roles...)
		sort.Strings(roles)
		rolesText := html.EscapeString(strings.Join(roles, ", "))
		if rolesText == "" {
			rolesText = "no roles"
		}

		_, err := io.WriteString(w, fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s - ParkOps</title>
  <style>
		:root{--bg:#f1f5f9;--panel:#fff;--border:#e2e8f0;--text:#0f172a;--muted:#64748b;--brand:#047857}
		body{background:var(--bg);color:var(--text);font-family:system-ui,-apple-system,Segoe UI,Roboto,Ubuntu,Cantarell,Noto Sans,sans-serif}
    .toast-container{position:fixed;top:1rem;right:1rem;z-index:9999;display:flex;flex-direction:column;gap:.5rem;pointer-events:none}
    .toast{pointer-events:auto;min-width:280px;max-width:380px;padding:.75rem 1rem;border-radius:.5rem;font-size:.875rem;font-weight:500;box-shadow:0 4px 12px rgba(0,0,0,.15);display:flex;align-items:center;gap:.5rem;animation:toast-in .3s ease-out,toast-out .3s ease-in 3.7s forwards}
    .toast-success{background:#065f46;color:#fff}
    .toast-error{background:#991b1b;color:#fff}
    .toast-info{background:#1e3a5f;color:#fff}
    @keyframes toast-in{from{opacity:0;transform:translateX(100%%)}to{opacity:1;transform:translateX(0)}}
    @keyframes toast-out{from{opacity:1}to{opacity:0}}
  </style>
</head>
<body class="min-h-screen bg-slate-100 text-slate-900">
  <div id="toastContainer" class="toast-container"></div>
  <div class="flex min-h-screen">
    <aside class="w-72 border-r border-slate-200 bg-white">
      <div class="border-b border-slate-200 px-6 py-5">
        <p class="text-xs font-semibold uppercase tracking-widest text-emerald-700">ParkOps</p>
        <p class="mt-2 text-sm text-slate-600">Operator Console</p>
      </div>
      <nav class="px-3 py-4">%s</nav>
    </aside>

    <div class="flex min-h-screen flex-1 flex-col">
      <header class="sticky top-0 z-20 border-b border-slate-200 bg-white/95 backdrop-blur">
        <div class="mx-auto flex w-full max-w-7xl items-center justify-between px-6 py-3">
          <div>
            <p class="text-sm font-semibold text-slate-900">ParkOps</p>
            <p class="text-xs text-slate-500">%s</p>
          </div>

          <div class="group relative">
            <button class="flex h-10 w-10 items-center justify-center rounded-full bg-emerald-700 text-sm font-semibold text-white">%s</button>
            <div class="invisible absolute right-0 top-11 w-72 rounded-xl border border-slate-200 bg-white p-4 opacity-0 shadow-lg transition group-hover:visible group-hover:opacity-100">
              <p class="text-sm font-semibold text-slate-900">%s</p>
              <p class="text-xs text-slate-600">@%s</p>
              <p class="mt-2 text-xs text-slate-500">Roles: %s</p>
              <form class="mt-3" method="post" action="/auth/logout" onsubmit="const btn=this.querySelector('button[type=submit]'); if(btn){btn.disabled=true; btn.textContent='Signing out...';}">
                <button class="w-full rounded-md bg-slate-900 px-3 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-60" type="submit">Logout</button>
              </form>
            </div>
          </div>
        </div>
      </header>

      <main class="mx-auto w-full max-w-7xl flex-1 p-6">
        %s
      </main>
    </div>
  </div>
  <script>
  (function(){
    var msgs={login_success:['Signed in successfully','success'],login_error:['Invalid username or password','error'],logout_success:['You have been signed out','success'],session_expired:['Session expired, please sign in again','info'],password_changed:['Password changed successfully','success'],forbidden:['You do not have permission','error']};
    function showToast(text,type){
      var c=document.getElementById('toastContainer');if(!c)return;
      var el=document.createElement('div');el.className='toast toast-'+(type||'info');
      var icon=type==='success'?'\u2713':type==='error'?'\u2717':'\u2139';
      el.textContent=icon+' '+text;c.appendChild(el);
      setTimeout(function(){el.remove()},4200);
    }
    var p=new URLSearchParams(window.location.search);var t=p.get('toast');
    if(t&&msgs[t]){showToast(msgs[t][0],msgs[t][1]);var u=new URL(window.location);u.searchParams.delete('toast');window.history.replaceState({},'',u)}
    window.parkopsToast=showToast;
  })();
  </script>
</body>
</html>`,
			html.EscapeString(pageTitle),
			navHTML,
			html.EscapeString(pageTitle),
			html.EscapeString(initials),
			html.EscapeString(fullName),
			html.EscapeString(user.Username),
			rolesText,
			contentHTML,
		))
		return err
	})
}

func buildNav(userRoles []string, currentPath string) string {
	items := []navItem{
		{Label: "Dashboard", Path: "/dashboard"},
		{Label: "Reservations", Path: "/reservations", Roles: []string{auth.RoleFacilityAdmin, auth.RoleDispatch}},
		{Label: "Capacity", Path: "/capacity"},
		{Label: "Members", Path: "/members", Roles: []string{auth.RoleFacilityAdmin, auth.RoleFleetManager}},
		{Label: "Vehicles", Path: "/vehicles", Roles: []string{auth.RoleFacilityAdmin, auth.RoleFleetManager}},
		{Label: "Drivers", Path: "/drivers", Roles: []string{auth.RoleFacilityAdmin, auth.RoleFleetManager}},
		{Label: "Facilities", Path: "/facilities", Roles: []string{auth.RoleFacilityAdmin}},
		{Label: "Lots", Path: "/lots", Roles: []string{auth.RoleFacilityAdmin}},
		{Label: "Zones", Path: "/zones", Roles: []string{auth.RoleFacilityAdmin}},
		{Label: "Rate Plans", Path: "/rate-plans", Roles: []string{auth.RoleFacilityAdmin}},
		{Label: "Campaigns", Path: "/campaigns"},
		{Label: "Segments", Path: "/segments"},
		{Label: "Analytics", Path: "/analytics"},
		{Label: "Notifications", Path: "/notifications"},
		{Label: "Audit Log", Path: "/audit", Roles: []string{auth.RoleAuditor, auth.RoleFacilityAdmin}},
		{Label: "Admin Users", Path: "/admin/users", Roles: []string{auth.RoleFacilityAdmin}},
	}

	var sb strings.Builder
	for _, item := range items {
		if !isNavVisible(userRoles, item.Roles) {
			continue
		}
		active := item.Path == currentPath
		if !active && item.Path != "/" && strings.HasPrefix(currentPath, item.Path+"/") {
			active = true
		}
		classes := "mb-1 block rounded-md px-3 py-2 text-sm text-slate-700 hover:bg-slate-100"
		if active {
			classes = "mb-1 block rounded-md bg-emerald-100 px-3 py-2 text-sm font-semibold text-emerald-900"
		}
		sb.WriteString(`<a class="` + classes + `" href="` + item.Path + `">` + html.EscapeString(item.Label) + `</a>`)
	}
	return sb.String()
}

func isNavVisible(userRoles []string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, r := range userRoles {
		for _, allow := range allowed {
			if r == allow {
				return true
			}
		}
	}
	return false
}

func initialsFor(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		r := []rune(parts[0])
		if len(r) == 0 {
			return ""
		}
		if len(r) == 1 {
			return strings.ToUpper(string(r[0]))
		}
		return strings.ToUpper(string(r[0]) + string(r[1]))
	}
	left := []rune(parts[0])
	right := []rune(parts[len(parts)-1])
	if len(left) == 0 || len(right) == 0 {
		return ""
	}
	return strings.ToUpper(string(left[0]) + string(right[0]))
}
