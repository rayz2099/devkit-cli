# Profile description and ds catalog

Agents cannot infer a Profile's purpose from `name` / `kind`. Optional `description` is the caption. `ds` and `-h ds` print the catalog without connecting and never print Url.

A required description was rejected so existing configs stay valid. Putting this on `doctor` was rejected: doctor connects; catalog must be free. Printing Url was rejected because passwords live in Url.
