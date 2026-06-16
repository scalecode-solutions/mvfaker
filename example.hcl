entity "customer" {
  field "name" {
    gen = "name.full"
  }
  field "email" {
    gen    = "internet.email"
    from   = "name" # coherence: email derives from the name
    unique = true   # dataset-layer uniqueness, no mutable set
  }
  field "age" {
    gen  = "number"
    min  = 18
    max  = 90
    dist = "normal"
    mode = 35
  }
  field "vip" {
    gen = "bool"
    p   = 0.1
  }
  field "country" {
    gen = "address.country"
  }
  field "city" {
    gen  = "address.city"
    from = "country" # coherent: city is within the country
  }
  field "phone" {
    gen  = "phone"
    from = "country" # coherent: dialing code matches the country
  }
  field "joined" {
    gen = "date"
    min = 2019
    max = 2024
  }
}

entity "order" {
  field "customer_id" {
    ref = "customer.id" # FK into the customer row space
  }
  field "total" {
    gen = "number"
    min = 1
    max = 500
  }
  field "note" {
    gen = "lorem.words"
    n   = 3
  }
}

dataset "demo" {
  counts = {
    customer = 1000
    order    = 5000
  }
}
