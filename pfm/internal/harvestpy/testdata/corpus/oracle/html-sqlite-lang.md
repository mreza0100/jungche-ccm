**Title:** SQL As Understood By SQLite
**Published:** 2024-04-01

---

Small. Fast. Reliable.

Choose any three.

Choose any three.

SQLite understands most of the standard SQL
language.  But it does [omit some features](omitted.html)
while at the same time
adding a few features of its own.  This document attempts to
describe precisely what parts of the SQL language SQLite does
and does not support.  A list of [SQL keywords](lang_keywords.html) is
also provided.  The SQL language syntax is described by
[syntax diagrams](syntaxdiagrams.html).

The following syntax documentation topics are available:

The routines [sqlite3_prepare_v2()](c3ref/prepare.html), [sqlite3_prepare()](c3ref/prepare.html),
[sqlite3_prepare16()](c3ref/prepare.html), [sqlite3_prepare16_v2()](c3ref/prepare.html),
[sqlite3_exec()](c3ref/exec.html), and [sqlite3_get_table()](c3ref/free_table.html) accept
an SQL statement list (sql-stmt-list) which is a semicolon-separated
list of statements.

Each SQL statement in the statement list is an instance of the following:

*This page was last updated on 2024-04-01 12:41:31Z *
