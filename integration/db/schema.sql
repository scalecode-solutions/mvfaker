-- Forum schema. Column order matches mvfaker's COPY output (id first, then the
-- fields in config order), so the generated COPY stream loads directly.

CREATE TABLE users (
    id      integer PRIMARY KEY,
    name    text NOT NULL,
    email   text NOT NULL UNIQUE,
    country text NOT NULL,
    city    text NOT NULL,
    joined  date NOT NULL
);

CREATE TABLE posts (
    id        integer PRIMARY KEY,
    author_id integer NOT NULL REFERENCES users (id),
    title     text NOT NULL,
    body      text NOT NULL,
    created   date NOT NULL
);

CREATE TABLE comments (
    id        integer PRIMARY KEY,
    post_id   integer NOT NULL REFERENCES posts (id),
    author_id integer NOT NULL REFERENCES users (id),
    body      text NOT NULL
);

CREATE INDEX ON posts (author_id);
CREATE INDEX ON comments (post_id);
CREATE INDEX ON comments (author_id);
