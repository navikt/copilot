"""parallel-interrupt checks that interrupting a script that uses
hack/parallel.sh stops its sections. It starts two 30s sections, sends
SIGINT or SIGTERM after a second and counts the sections still running.
Python starts the script, not bash: a shell's background job ignores SIGINT,
which would test the harness instead. Run from the repo root:

    python3 hack/probes/parallel-interrupt.py [path/to/parallel.sh]
"""
import os, signal, subprocess, sys, tempfile, time

lib = os.path.abspath(sys.argv[1] if len(sys.argv) > 1 else "hack/parallel.sh")
tag = "sleep 30.%d" % os.getpid()  # an odd duration, to find these and no others
script = tempfile.NamedTemporaryFile("w", suffix=".sh", delete=False)
script.write(f"""source {lib}
section a sh -c '{tag}'
section b sh -c '{tag}'
wait_sections
""")
script.close()

def running():
    out = subprocess.run(["pgrep", "-f", tag], capture_output=True, text=True).stdout
    return len(out.split())

for sig in (signal.SIGINT, signal.SIGTERM):
    p = subprocess.Popen(["bash", script.name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(1)
    before = running()
    t = time.time()
    p.send_signal(sig)
    p.wait()
    took = time.time() - t
    time.sleep(0.5)
    left = running()
    print(f"{sig.name}: sections running before {before}, exit {p.returncode} after {took:.1f}s, still running {left}")
    subprocess.run(["pkill", "-f", tag])
os.unlink(script.name)
