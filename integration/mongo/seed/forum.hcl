# mvfaker config for the forum schema. Field order MUST match the table column
# order (after the implicit id), so the COPY stream lines up.

entity "users" {
  field "name" {
    gen = "name.full"
  }
  field "email" {
    gen    = "internet.email"
    from   = "name"   # email derived from the name
    unique = true     # matches the UNIQUE constraint on users.email
  }
  field "country" {
    gen = "address.country"
  }
  field "city" {
    gen  = "address.city"
    from = "country"  # city is within the country
  }
  field "joined" {
    gen = "date"
    min = 2019
    max = 2024
  }
}

entity "posts" {
  field "author_id" {
    ref = "users.id"  # FK → users
  }
  field "title" {
    gen = "lorem.words"
    n   = 5
  }
  field "body" {
    gen = "lorem.words"
    n   = 30
  }
  field "created" {
    gen = "date"
    min = 2022
    max = 2024
  }
}

entity "comments" {
  field "post_id" {
    ref = "posts.id"  # FK → posts
  }
  field "author_id" {
    ref = "users.id"  # FK → users
  }
  field "body" {
    gen = "lorem.words"
    n   = 12
  }
}

dataset "forum" {
  counts = {
    users    = 5000
    posts    = 20000
    comments = 80000
  }
}
