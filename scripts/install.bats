#!/usr/bin/env bats

setup() {
  export TMP_DIR="$(mktemp -d)"
  export PATH="${TMP_DIR}:${PATH}"
  
  # Create a bin directory for our mocks
  export MOCK_BIN="${TMP_DIR}/bin"
  mkdir -p "${MOCK_BIN}"
  export PATH="${MOCK_BIN}:${PATH}"

  cat <<'EOF' > "${MOCK_BIN}/curl"
#!/bin/bash
out_file=""
for i in "$@"; do
  if [[ "$prev" == "-o" ]]; then
    out_file="$i"
  fi
  prev="$i"
done

write_out() {
  if [[ -n "$out_file" ]]; then
    echo -e "$1" > "$out_file"
  else
    echo -e "$1"
  fi
}

if [[ "$*" == *"api.github.com"* ]]; then
  write_out '{"tag_name": "nav-pilot/2026.01.01-mock"}'
  exit 0
elif [[ "$*" == *"SHA256SUMS"* ]]; then
  write_out "dummyhash  nav-pilot-linux-amd64\ndummyhash  nav-pilot-darwin-arm64"
  exit 0
elif [[ "$*" == *".sh"* ]]; then
  write_out "# Mocked script download"
  exit 0
fi
write_out "Mocked curl download"
exit 0
EOF
  chmod +x "${MOCK_BIN}/curl"

  # Mock chmod so we don't actually modify the system (or if file doesn't exist)
  cat <<'EOF' > "${MOCK_BIN}/chmod"
#!/bin/bash
echo "mock chmod $1 $2"
exit 0
EOF
  chmod +x "${MOCK_BIN}/chmod"

  # Default mock for sha256sum
  cat <<'EOF' > "${MOCK_BIN}/sha256sum"
#!/bin/bash
echo "dummyhash  filename"
exit 0
EOF
  chmod +x "${MOCK_BIN}/sha256sum"

  # Default mock for uname (Linux x86_64)
  cat <<'EOF' > "${MOCK_BIN}/uname"
#!/bin/bash
if [[ "$1" == "-s" ]]; then
  echo "Linux"
elif [[ "$1" == "-m" ]]; then
  echo "x86_64"
fi
exit 0
EOF
  chmod +x "${MOCK_BIN}/uname"

  # Mock mv so we don't actually modify the system
  cat <<'EOF' > "${MOCK_BIN}/mv"
#!/bin/bash
echo "mock mv $1 to $2"
exit 0
EOF
  chmod +x "${MOCK_BIN}/mv"

  export SCRIPT="${BATS_TEST_DIRNAME}/install.sh"
}

teardown() {
  rm -rf "${TMP_DIR}"
}

@test "installs via brew on macOS if brew is available" {
  cat <<'EOF' > "${MOCK_BIN}/uname"
#!/bin/bash
echo "Darwin"
EOF
  
  cat <<'EOF' > "${MOCK_BIN}/brew"
#!/bin/bash
echo "mock brew $*"
exit 0
EOF
  chmod +x "${MOCK_BIN}/brew"

  # Also mock nav-pilot so it doesn't fail the version check
  cat <<'EOF' > "${MOCK_BIN}/nav-pilot"
#!/bin/bash
echo "mock.version"
exit 0
EOF
  chmod +x "${MOCK_BIN}/nav-pilot"

  run bash "$SCRIPT"
  
  [ "$status" -eq 0 ]
  [[ "${lines[0]}" == "→ Installing via Homebrew..." ]]
}

@test "downloads linux binary if OS is Linux" {
  run bash "$SCRIPT" --dir "${TMP_DIR}/install-dest"
  
  [ "$status" -eq 0 ]
  # Check that it fetched the latest release and correctly resolved the linux-amd64 asset
  [[ "$output" == *"Fetching latest nav-pilot release"* ]]
  [[ "$output" == *"nav-pilot nav-pilot/2026.01.01-mock (linux/amd64)"* ]]
  [[ "$output" == *"Downloading nav-pilot-linux-amd64"* ]]
  [[ "$output" == *"Installed nav-pilot to ${TMP_DIR}/install-dest/nav-pilot"* ]]
}

@test "downloads darwin binary if OS is macOS and --no-brew is used" {
  cat <<'EOF' > "${MOCK_BIN}/uname"
#!/bin/bash
if [[ "$1" == "-s" ]]; then echo "Darwin"; elif [[ "$1" == "-m" ]]; then echo "arm64"; fi
exit 0
EOF

  run bash "$SCRIPT" --dir "${TMP_DIR}/install-dest" --no-brew
  
  [ "$status" -eq 0 ]
  [[ "$output" == *"nav-pilot nav-pilot/2026.01.01-mock (darwin/arm64)"* ]]
  [[ "$output" == *"Downloading nav-pilot-darwin-arm64"* ]]
}

@test "fails if checksum does not match" {
  # Modify sha256sum to return a bad hash
  cat <<'EOF' > "${MOCK_BIN}/sha256sum"
#!/bin/bash
echo "badhash  filename"
exit 0
EOF

  run bash "$SCRIPT" --dir "${TMP_DIR}/install-dest"
  
  [ "$status" -eq 1 ]
  [[ "$output" == *"Checksum mismatch!"* ]]
  [[ "$output" == *"Expected: dummyhash"* ]]
  [[ "$output" == *"Got:      badhash"* ]]
}

@test "installs cplt and rtk dependencies" {
  run bash "$SCRIPT" --dir "${TMP_DIR}/install-dest"
  
  [ "$status" -eq 0 ]
  [[ "$output" == *"Installing cplt (sandbox)"* ]]
  [[ "$output" == *"Installed cplt"* ]]
  [[ "$output" == *"Installing rtk (token optimizer)"* ]]
  [[ "$output" == *"Installed rtk"* ]]
}
