package hook

import (
	"encoding/json"
	"testing"
)

func TestRiskyCommand(t *testing.T) {
	tests := []struct{ cmd, want string }{
		{"kubectl delete deployment foo -n team", "kubectl"},
		{"kubectl -n team apply -f app.yaml", "kubectl"},
		{"kubectl rollout restart deployment/foo", "kubectl"},
		{"kubectl scale deploy/foo --replicas=0", "kubectl"},
		{"KUBECONFIG=~/.kube/prod kubectl delete pod x", "kubectl"},
		{"kubectl get pods -n team", ""},
		{"kubectl logs deploy/foo --tail=50", ""},
		{"kubectl rollout status deployment/foo", ""},
		{"kubectl describe pod delete-me", ""},
		{"nais postgres prepare --team t app", "nais"},
		{"nais device status", ""},
		{"gcloud sql instances delete db", "gcloud"},
		{"gcloud projects add-iam-policy-binding p --member=x --role=y", "gcloud"},
		{"gcloud compute instances list", ""},
		{"helm upgrade --install foo ./chart", "helm"},
		{"helm template ./chart", ""},
		{"terraform apply -auto-approve", "terraform"},
		{"tofu destroy", "terraform"},
		{"terraform plan", ""},
		{"rm -rf node_modules", "rm"},
		{"rm -fR build", "rm"},
		{"rm --recursive dist", "rm"},
		{"rm -f a.txt", ""},
		{"sudo rm -rf /var/lib/x", "rm"},
		{"find . -name '*.tmp' | xargs rm -rf", "rm"},
		{"sudo -u root rm -rf /var/lib/x", "rm"},
		{"env -u FOO rm -rf build", "rm"},
		{"xargs -n 1 rm -rf < dirs.txt", "rm"},
		{"time -f %e kubectl delete pod x", "kubectl"},
		{`bash -c "rm -rf /tmp/x"`, "rm"},
		{"cd /tmp && rm -r old", "rm"},
		{"git push --force origin main", "git"},
		{"git push -f", "git"},
		{"git push origin +main", "git"},
		{"git push --force-with-lease", "git"},
		{"git -C repo push -uf origin x", "git"},
		{"git push origin feature", ""},
		{"git reset --hard HEAD~1", "git"},
		{"git reset --soft HEAD~1", ""},
		{"git clean -fdx", "git"},
		{"git clean -n", ""},
		{"git status", ""},
		{"dd if=/dev/zero of=/dev/disk2 bs=1m", "disk"},
		{"mkfs.ext4 /dev/sdb1", "disk"},
		{`psql -c "DROP TABLE users"`, "sql"},
		{`echo 'drop database app;' | psql`, "sql"},
		{`psql -c "DROP INDEX users_email"`, "sql"},
		{`psql -c "TRUNCATE users"`, "sql"},
		{`psql -c "SELECT * FROM backdrop"`, ""},
		{`grep -r "DROP TABLE" migrations/`, ""},
		{"ls -la", ""},
		{"go test ./...", ""},
	}
	for _, tt := range tests {
		if got := RiskyCommand(tt.cmd); got != tt.want {
			t.Errorf("RiskyCommand(%q) = %q, want %q", tt.cmd, got, tt.want)
		}
	}
}

func TestShellCommand(t *testing.T) {
	cmd, desc, ok := ShellCommand("Bash", json.RawMessage(`{"command":"ls","description":"List"}`))
	if !ok || cmd != "ls" || desc != "List" {
		t.Errorf("bash: %q %q %v", cmd, desc, ok)
	}
	if _, _, ok := ShellCommand("edit", json.RawMessage(`{"command":"ls"}`)); ok {
		t.Error("an edit read as a shell call")
	}
	if _, _, ok := ShellCommand("bash", json.RawMessage(`{}`)); ok {
		t.Error("an empty command read as one")
	}
}
