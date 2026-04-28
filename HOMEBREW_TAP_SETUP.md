# Homebrew Tap Setup

The Homebrew formula is auto-generated and pushed by `goreleaser` on every
release tag. You only need to do this **once**.

## One-time setup

1. **Create the tap repo on GitHub.** It must be named `homebrew-tap` and live
   under the same owner as the main repo:

   ```
   github.com/zigamedved/homebrew-tap
   ```

   Initialize it with a README and a `Formula/` directory.

2. **Create a fine-grained personal access token** with write access to the
   tap repo only:

   - Go to https://github.com/settings/tokens?type=beta
   - Repository access: only `zigamedved/homebrew-tap`
   - Permissions: `Contents: Read and write`, `Metadata: Read-only`
   - Copy the token.

3. **Add the token to the main repo's secrets** as `HOMEBREW_TAP_GITHUB_TOKEN`:

   ```
   github.com/zigamedved/dotenv-doctor → Settings → Secrets → Actions
   New secret: HOMEBREW_TAP_GITHUB_TOKEN
   ```

4. **Tag a release.** When you push a tag matching `v*`, the release workflow
   builds the binaries, pushes to GitHub Releases, and pushes a generated
   formula to `zigamedved/homebrew-tap/Formula/dotenv-doctor.rb`.

## How users install after that

```bash
brew tap zigamedved/tap
brew install dotenv-doctor

# or in one line:
brew install zigamedved/tap/dotenv-doctor
```

## Cutting a release

```bash
# Make sure CI is green on main, then:
git tag v0.1.0
git push origin v0.1.0
```

The release workflow does the rest.
