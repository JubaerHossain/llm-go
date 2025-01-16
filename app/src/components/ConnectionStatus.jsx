import { FaCheckCircle, FaTimesCircle } from "react-icons/fa";
import PropTypes from 'prop-types';

function ConnectionStatus({ isConnected }) {
  return (
    <span
      className={`connection-status ${
        isConnected ? "connected" : "disconnected"
      }`}
    >
      {isConnected ? (
        <>
          <FaCheckCircle style={{ color: "green", marginRight: "5px" }} />
          Connected
        </>
      ) : (
        <>
          <FaTimesCircle style={{ color: "red", marginRight: "5px" }} />
          Disconnected
        </>
      )}
    </span>
  );
}
ConnectionStatus.propTypes = {
  isConnected: PropTypes.bool.isRequired,
};

export default ConnectionStatus;
