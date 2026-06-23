-- Normalize legacy plan/eval paths in metadata; add runs.eval_file_path for build baseline.

UPDATE plans SET path = 'docs/plans/' || substring(path from '[^/\\]+$')
  WHERE path NOT LIKE 'docs/%' AND (path LIKE '.matrix/%' OR path LIKE '.spec/%');

UPDATE artifacts SET path = 'docs/evaluations/' || substring(path from '[^/\\]+$')
  WHERE path NOT LIKE 'docs/%' AND (path LIKE '.matrix/%' OR path LIKE '.spec/%');

UPDATE artifacts SET plan_path = 'docs/plans/' || substring(plan_path from '[^/\\]+$')
  WHERE plan_path <> '' AND plan_path NOT LIKE 'docs/%';

UPDATE runs SET file_path = 'docs/plans/' || substring(file_path from '[^/\\]+$')
  WHERE file_path <> '' AND file_path NOT LIKE 'docs/%';

ALTER TABLE runs ADD COLUMN IF NOT EXISTS eval_file_path VARCHAR(512) DEFAULT '';
