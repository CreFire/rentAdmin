const { request } = require("./http");

async function createOrder(token, tenantBillId, amountFen) {
  return request({
    url: "/api/mp/pay/orders",
    method: "POST",
    token,
    data: { tenantBillId, amountFen },
  });
}

async function queryOrder(token, outTradeNo) {
  return request({
    url: `/api/mp/pay/orders/${outTradeNo}`,
    method: "GET",
    token,
  });
}

async function notifyMockSuccess(outTradeNo) {
  return request({
    url: "/api/mp/pay/notify",
    method: "POST",
    data: {
      outTradeNo,
      transactionId: `mock_tx_${Date.now()}`,
      success: true,
      paidAt: new Date().toISOString(),
      eventId: `mock_evt_${Date.now()}`,
    },
  });
}

async function recordSubscribe(token, templateId) {
  return request({
    url: "/api/mp/subscribe/record",
    method: "POST",
    token,
    data: { templateId },
  });
}

module.exports = {
  createOrder,
  queryOrder,
  notifyMockSuccess,
  recordSubscribe,
};
