CREATE TABLE questions (
  id INTEGER PRIMARY KEY,
  asked INTEGER NOT NULL,
  question TEXT NOT NULL,
  answered INTEGER,
  answer TEXT
) STRICT;

CREATE INDEX questions_answered ON questions (answered, id) WHERE answered IS NOT NULL;
