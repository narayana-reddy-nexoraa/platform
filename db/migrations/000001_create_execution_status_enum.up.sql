CREATE TYPE execution_status AS ENUM (
    'CREATED',
    'CLAIMED',
    'RUNNING',
    'SUCCEEDED',
    'FAILED',
    'CANCELED',
    'TIMED_OUT'
);
