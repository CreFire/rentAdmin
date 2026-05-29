const { request } = require("./http");

async function fetchBills(token) {
  return request({
    url: "/api/mp/bills",
    method: "GET",
    token,
  });
}

module.exports = { fetchBills };
