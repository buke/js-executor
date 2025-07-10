async rpcRequest => {
  if (rpcRequest.args == undefined) {
    rpcRequest.args = [];
  }

  let rpcResponse = { id: rpcRequest.id, context: rpcRequest.context };

  // global service
  if (rpcRequest.service.split('.').length == 1) {
    rpcRequest.service = 'globalThis.' + rpcRequest.service;
  }

  let cls = (0, eval)(rpcRequest.service.split('.').slice(0, -1).join('.'));
  let methodName = rpcRequest.service.split('.').slice(-1).join('');

  if (typeof cls[methodName] === 'function') {
    // static method
    rpcResponse.result = cls[methodName](...rpcRequest.args);
  } else {
    // instance method
    let obj = new cls(...rpcRequest.args[0]);
    rpcResponse.result = obj[methodName](...rpcRequest.args.slice(1));
  }

  if (rpcResponse.result instanceof Promise) {
    rpcResponse.result = await rpcResponse.result;
  }

  return rpcResponse;
};
