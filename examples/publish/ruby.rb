# frozen_string_literal: true

require "json"
require "net/http"

token = "eyJhbGciOiJIUzI1NiIsInR5cCI6ImF0K2p3dCJ9.eyJhdWQiOiJodHRwczovL2xvY2FsaG9zdC8ud2VsbC1rbm93bi9tZXJjdXJlIiwiYXV0aG9yaXphdGlvbl9kZXRhaWxzIjpbeyJhY3Rpb25zIjpbInB1Ymxpc2giXSwidG9waWNzIjpbeyJtYXRjaCI6IioifV0sInR5cGUiOiJodHRwczovL21lcmN1cmUucm9ja3MvYXV0aG9yaXphdGlvbi1kZXRhaWwifSx7ImFjdGlvbnMiOlsic3Vic2NyaWJlIl0sInRvcGljcyI6W3sibWF0Y2giOiIqIn1dLCJ0eXBlIjoiaHR0cHM6Ly9tZXJjdXJlLnJvY2tzL2F1dGhvcml6YXRpb24tZGV0YWlsIn1dLCJleHAiOjQxMDI0NDQ4MDAsImlzcyI6Imh0dHBzOi8vbG9jYWxob3N0In0.VO0-PRjJ2MGOrMk2HxlrBv217pB7hyLxLIQUGgSfyXs"

Net::HTTP.start("localhost", 8080) do |http|
  req = Net::HTTP::Post.new("/.well-known/mercure")
  req["Authorization"] = "Bearer #{token}"
  req.form_data = {
    topic: "https://localhost/demo/books/1.jsonld",
    data: { key: :value }.to_json
  }
  req = http.request(req)
  puts req.body
end
