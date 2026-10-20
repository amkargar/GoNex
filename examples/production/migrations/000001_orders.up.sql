CREATE TABLE orders (
 id TEXT PRIMARY KEY,
 item TEXT NOT NULL,
 quantity INTEGER NOT NULL CHECK (quantity BETWEEN 1 AND 1000),
 status TEXT NOT NULL CHECK (status IN ('pending','completed')),
 created_at TIMESTAMP NOT NULL
);
CREATE INDEX orders_status_created_idx ON orders(status, created_at, id);
