CREATE TABLE product (
    price bigint,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    code text UNIQUE UNIQUE,
    id SERIAL PRIMARY KEY
);
