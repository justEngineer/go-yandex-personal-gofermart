CREATE TABLE users (
    id uuid default gen_random_uuid() PRIMARY KEY,
    login varchar(100) not null,
    password_hash varchar(100) not null,
    CONSTRAINT unique_login UNIQUE(login)
);

CREATE TABLE orders (
    id uuid default gen_random_uuid() PRIMARY KEY,
    user_id uuid NOT NULL,
    status varchar(15),
    amount DECIMAL,
    external_id varchar(100) NOT NULL,
    registered_at timestamp default now() NOT NULL,
    CONSTRAINT fk_user FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE TABLE withdrawal (
    id uuid default gen_random_uuid()  PRIMARY KEY,
    user_id uuid NOT NULL,
    amount DECIMAL NOT NULL,
    external_id varchar(100) NOT NULL,
    registered_at timestamp default now() NOT NULL,
    CONSTRAINT fk_user FOREIGN KEY(user_id) REFERENCES users(id)
);