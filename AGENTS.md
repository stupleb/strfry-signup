# AGENTS.md

`strfry-signup` is the sign-up service that the strfry StartOS package ([stupleb/strfry-startos](https://github.com/stupleb/strfry-startos)) carries as a git submodule and builds in its Dockerfile. `README.md` is the reference for what it does and for its configuration contract. **Keep both files up to date in the same change as the code they describe.**

- Vet, test and build in `golang:1.27-alpine`, the image the package builds with: `docker run --rm -v "$PWD":/src -w /src golang:1.27-alpine sh -c 'go vet ./... && go test ./...'`.
- The environment variables, both HTTP APIs and the `state.json` and `blocked.json` formats are what the package depends on. Change one only together with the package, and keep every `state.json` an earlier version wrote loadable: the package reads `members` from it and drops all members if an entry stops matching.
- Never raise the `go` line in `go.mod` past the package Dockerfile's `golang:<minor>-alpine` image; move that image first.
- Put every string the page, `app.js` or the public API shows in `i18n.go`, in all five locales. `go test` fails on a missing key or a placeholder that differs from English. Leave admin API errors and logs in English: the package shows admin errors inside its own translated message.
- Pass data to `app.js` through `data-` attributes on its `<script>` tag: the content security policy allows no inline script or style.
