package web

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

func LoginPage() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_ = ctx
		_, err := io.WriteString(w, `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ParkOps Login</title>
  <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="min-h-screen bg-slate-100 text-slate-900">
  <main class="mx-auto flex min-h-screen max-w-md items-center p-6">
    <section class="w-full rounded-xl bg-white p-8 shadow-sm ring-1 ring-slate-200">
      <p class="text-sm font-medium uppercase tracking-widest text-emerald-700">ParkOps</p>
      <h1 class="mt-2 text-3xl font-semibold">Sign in</h1>
      <p class="mt-1 text-sm text-slate-600">Local operations console</p>
      <form class="mt-8 space-y-4" method="post" action="/auth/login">
        <label class="block">
          <span class="mb-1 block text-sm font-medium">Username</span>
          <input class="w-full rounded-md border border-slate-300 px-3 py-2" type="text" name="username" autocomplete="username" required>
        </label>
        <label class="block">
          <span class="mb-1 block text-sm font-medium">Password</span>
          <input class="w-full rounded-md border border-slate-300 px-3 py-2" type="password" name="password" autocomplete="current-password" required>
        </label>
        <button class="w-full rounded-md bg-emerald-700 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-800" type="submit">Sign in</button>
      </form>
    </section>
  </main>
</body>
</html>`)
		return err
	})
}
