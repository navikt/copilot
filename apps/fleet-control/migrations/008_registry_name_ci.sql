-- Registry names are unique regardless of case (#2853): Foo/x and foo/x
-- would show as two servers to a client that compares case-insensitively.
DO $$
DECLARE dup TEXT;
BEGIN
    SELECT string_agg(n, ', ') INTO dup FROM (
        SELECT lower(registry_name) AS n FROM servers
        WHERE registry_name IS NOT NULL GROUP BY 1 HAVING count(*) > 1) d;
    IF dup IS NOT NULL THEN
        RAISE EXCEPTION 'registry names differ only in case: %. Rename or revoke one of each before this migration can run.', dup;
    END IF;
END $$;

CREATE UNIQUE INDEX servers_registry_name_lower ON servers (lower(registry_name));
