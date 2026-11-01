// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title ArcpayTreasury
 * @notice Native Treasury & Trade Escrow Engine on Circle's Arc Layer-1.
 * On Arc L1, transaction fees are natively settled in USDC.
 */
contract ArcpayTreasury {
    address public immutable protocolOwner;
    uint256 public constant PROTOCOL_FEE_BPS = 50; // 0.50% take rate

    enum EscrowStatus { PENDING, FUNDED, DISPATCHED, RELEASED, REFUNDED }

    struct TradeEscrow {
        bytes32 invoiceId;
        address importer;
        address supplier;
        uint256 amountUnits; // 1 USDC = 1,000,000 micro-units
        EscrowStatus status;
        uint256 createdAt;
    }

    mapping(bytes32 => TradeEscrow) public tradeEscrows;

    event PaymentSettled(
        bytes32 indexed reference,
        address indexed sender,
        address indexed recipient,
        uint256 amountUnits,
        uint256 feeUnits
    );

    event EscrowFunded(bytes32 indexed invoiceId, address indexed importer, address indexed supplier, uint256 amountUnits);
    event EscrowReleased(bytes32 indexed invoiceId, address indexed supplier, uint256 amountUnits);
    event EscrowRefunded(bytes32 indexed invoiceId, address indexed importer, uint256 amountUnits);

    modifier onlyOwner() {
        require(msg.sender == protocolOwner, "Unauthorized: only protocol owner");
        _;
    }

    constructor() {
        protocolOwner = msg.sender;
    }

    /**
     * @notice Records and settles an onchain payment with automatic 0.50% fee diversion.
     */
    function settlePayment(
        bytes32 reference,
        address sender,
        address recipient,
        uint256 amountUnits
    ) external onlyOwner {
        require(recipient != address(0), "Invalid recipient");
        require(amountUnits > 0, "Amount must be greater than zero");

        uint256 feeUnits = (amountUnits * PROTOCOL_FEE_BPS) / 10000;
        uint256 settlementUnits = amountUnits - feeUnits;

        emit PaymentSettled(reference, sender, recipient, settlementUnits, feeUnits);
    }

    /**
     * @notice Locks funds in escrow for an importer paying overseas suppliers (Banex/Alaba -> China/Dubai).
     */
    function lockTradeEscrow(
        bytes32 invoiceId,
        address importer,
        address supplier,
        uint256 amountUnits
    ) external onlyOwner {
        require(tradeEscrows[invoiceId].status == EscrowStatus.PENDING, "Escrow already exists");
        
        tradeEscrows[invoiceId] = TradeEscrow({
            invoiceId: invoiceId,
            importer: importer,
            supplier: supplier,
            amountUnits: amountUnits,
            status: EscrowStatus.FUNDED,
            createdAt: block.timestamp
        });

        emit EscrowFunded(invoiceId, importer, supplier, amountUnits);
    }

    /**
     * @notice Releases escrowed digital dollars to supplier upon proof of dispatch / bill of lading.
     */
    function releaseTradeEscrow(bytes32 invoiceId) external onlyOwner {
        TradeEscrow storage escrow = tradeEscrows[invoiceId];
        require(escrow.status == EscrowStatus.FUNDED || escrow.status == EscrowStatus.DISPATCHED, "Escrow not ready");

        escrow.status = EscrowStatus.RELEASED;
        emit EscrowReleased(invoiceId, escrow.supplier, escrow.amountUnits);
    }
}
