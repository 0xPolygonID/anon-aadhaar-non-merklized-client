import Web3 from "web3";
import { Id } from "@iden3/js-iden3-core";
import { AnonAadhaarProof, packGroth16Proof } from "@anon-aadhaar/core";
import abi from "./abi.json";

const nullifierSeed = process.env.NEXT_PUBLIC_NULLIFIER_SEED!;
const contractABI = abi.abi;

const amoyChainID: bigint = BigInt(80002);
export const switchNetwork = async () => {
  const web3 = new Web3(window.ethereum);
  const networkId = await web3.eth.net.getId();
  if (networkId !== amoyChainID) {
    try {
      await window.ethereum.request({
        method: 'wallet_switchEthereumChain',
        params: [{ chainId: `0x${amoyChainID.toString(16)}` }],
      });
    } catch (switchError: any) {
      if (switchError.code === 4902) {
        throw new Error('This network is not available in your MetaMask, please add it.');
      } else {
        throw new Error('Failed to switch network');
      }
    }
  }
};

export const issueCredential = async (
  contractAddress: string,
  userId: Id,
  anonAadhaarProof: AnonAadhaarProof
) => {
  const web3 = new Web3(window.ethereum);
  const accounts = await web3.eth.getAccounts();
  const from = accounts[0];
  const onchainNonMerklizedIssuer = new web3.eth.Contract(
    contractABI,
    contractAddress
  );
  const packedGroth16Proof = packGroth16Proof(anonAadhaarProof.groth16Proof);

  try {
    const gasPrice = await web3.eth.getGasPrice();
    const priorityGasPrice = (gasPrice * BigInt(150)) / BigInt(100);

    await onchainNonMerklizedIssuer.methods
      .issueCredential(
        userId.bigInt(),
        nullifierSeed,
        anonAadhaarProof.nullifier,
        anonAadhaarProof.timestamp,
        from,
        [
          anonAadhaarProof.ageAbove18,
          anonAadhaarProof.gender,
          anonAadhaarProof.pincode,
          anonAadhaarProof.state,
        ],
        packedGroth16Proof
      )
      .send({
        from,
        maxPriorityFeePerGas: priorityGasPrice.toString(),
      });
  } catch (e) {
    console.error("Error estimating gas or sending transaction:", e);
    throw e;
  }
};

export const getUserCredentialIds = async (
  contractAddress: string,
  userId: Id
): Promise<Array<string>> => {
  const web3 = new Web3(window.ethereum);
  const contract = new web3.eth.Contract(contractABI, contractAddress);
  const result = await contract.methods
    .getUserCredentialIds(userId.bigInt())
    .call();
  if (!Array.isArray(result)) {
    throw new Error("Invalid result");
  }
  return result.map((id: number) => id.toString());
};

export const getCredential = async (
  contractAddress: string,
  userId: Id,
  credentialId: string
): Promise<string> => {
  const web3 = new Web3(window.ethereum);
  const functionAbi = contractABI.find(
    (func) => func.name === "getCredential" && func.type === "function"
  );
  if (!functionAbi) {
    throw new Error("Function ABI not found");
  }
  const data = web3.eth.abi.encodeFunctionCall(functionAbi, [
    userId.bigInt(),
    credentialId,
  ]);
  const transactionObject = {
    to: contractAddress.startsWith("0x")
      ? contractAddress
      : "0x" + contractAddress,
    data: data,
  };
  const resultHex = await web3.eth.call(transactionObject);
  return resultHex;
};
