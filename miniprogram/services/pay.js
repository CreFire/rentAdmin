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

// 仅 mock_mode 可用：后端会校验 token 对应的用户是否为订单所有者。
async function notifyMockSuccess(token, outTradeNo) {
  return request({
    url: "/api/mp/pay/notify",
    method: "POST",
    token,
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
