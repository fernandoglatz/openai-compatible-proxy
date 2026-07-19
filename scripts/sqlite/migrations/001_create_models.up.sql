CREATE TABLE models (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  name TEXT NOT NULL,
  object TEXT,
  type TEXT,
  publisher TEXT,
  arch TEXT,
  compatibility_type TEXT,
  quantization TEXT,
  state TEXT,
  max_context_length INTEGER NOT NULL DEFAULT 0,
  display_name TEXT,
  size_bytes INTEGER NOT NULL DEFAULT 0,
  params_string TEXT,
  capabilities TEXT NOT NULL DEFAULT '[]',
  loaded_instance_ids TEXT NOT NULL DEFAULT '[]'
);
