import React, { useEffect, useRef, useState } from "react";
import { FaPaperPlane, FaMicrophone } from "react-icons/fa";
import { v4 as uuidv4 } from "uuid";
import "./App.css";

function App() {
    const [query, setQuery] = useState("");
    const [messages, setMessages] = useState([]);
    const [isConnected, setIsConnected] = useState(false);
    const socketRef = useRef(null);
    const [isSending, setIsSending] = useState(false);
    const messagesEndRef = useRef(null);
    const currentMessageRef = useRef("");
    const currentMessageIdRef = useRef("");
    const localStorageKey = "chatHistory";
     const [isListening, setIsListening] = useState(false);
    const recognitionRef = useRef(null);
    const retryIntervalRef = useRef(null)
     const connectionAttemptRef = useRef(0)
    const maxConnectionAttempts = 3; // Maximum retry attempts
    const retryDelay = 3000; // 3 second delay before retry

    var count = 0;
    useEffect(() => {
      const savedMessages = localStorage.getItem(localStorageKey);
        if (savedMessages) {
            setMessages(JSON.parse(savedMessages));
        }

          const connectWebSocket = () => {
            if (
                socketRef.current &&
                socketRef.current.readyState === WebSocket.OPEN
            ) {
                return;
            }

             const wsURL = `ws://localhost:8080/chat`;
              socketRef.current = new WebSocket(wsURL);


            socketRef.current.onopen = () => {
                setIsConnected(true);
                console.log("Connected to WebSocket");
                connectionAttemptRef.current = 0; // Reset retry attempts on successful connection
               if(retryIntervalRef.current){
                     clearInterval(retryIntervalRef.current);
                    retryIntervalRef.current = null;
               }
            };

             socketRef.current.onmessage = (event) => {
                  console.log('Message received:', event.data);
                 setIsSending(false);
                try {
                     const data = JSON.parse(event.data);
                    if (data.answer) {
                       if(currentMessageIdRef.current === ''){
                            currentMessageIdRef.current = uuidv4()
                            currentMessageRef.current = ''
                         }

                         currentMessageRef.current += data.answer

                         count++;
                         console.log(count) ;
                         

                         setMessages(prevMessages => {
                            const updatedMessages =  prevMessages.map(message => {
                                if(message.id === currentMessageIdRef.current){
                                    return {...message, text: currentMessageRef.current, sender:"bot"}
                                }
                                return message
                            })
                           localStorage.setItem(localStorageKey, JSON.stringify(updatedMessages));
                            return updatedMessages
                        })
                     } else if (data.error) {
                         const newMessage = {
                            id: uuidv4(),
                            sender: "bot",
                            text: `<p style="color:red;">Error: ${data.error}</p>`
                        };
                        setMessages(prevMessages => {
                             const updatedMessages = [...prevMessages, newMessage]
                             localStorage.setItem(localStorageKey, JSON.stringify(updatedMessages));
                             return updatedMessages
                        });
                    }
                } catch (e) {
                    const newMessage = {
                       id: uuidv4(),
                         sender: "bot",
                         text: `<p style="color:red;">Error: ${event.data}</p>`
                     };
                      setMessages(prevMessages => {
                            const updatedMessages = [...prevMessages, newMessage]
                             localStorage.setItem(localStorageKey, JSON.stringify(updatedMessages));
                            return updatedMessages;
                       });
                     console.error("Error parsing message:", event.data, e)
                }

            };


          socketRef.current.onerror = (error) => {
              console.error('WebSocket error:', error)
               setIsConnected(false);
                 if (connectionAttemptRef.current < maxConnectionAttempts) {
                    if (!retryIntervalRef.current) { // Prevent multiple intervals
                      retryIntervalRef.current = setInterval(() => {
                        console.log('Attempting to reconnect to WebSocket.');
                        connectWebSocket();
                        connectionAttemptRef.current++;
                     }, retryDelay);
                    }
                 } else {
                      console.error('Max connection attempts reached, WebSocket connection not restored.');
                        if(retryIntervalRef.current){
                         clearInterval(retryIntervalRef.current);
                            retryIntervalRef.current = null;
                         }
                   }
            };


          socketRef.current.onclose = () => {
             setIsSending(false);
              console.log("Disconnected from WebSocket");
               setIsConnected(false);
          };
        }

         connectWebSocket();
        return () => {
            if(socketRef.current){
                socketRef.current.close();
            }
             if (retryIntervalRef.current) {
                  clearInterval(retryIntervalRef.current);
                  retryIntervalRef.current = null;
             }
         };
    }, []);


    useEffect(() => {
        if (messagesEndRef.current) {
            messagesEndRef.current.scrollIntoView({ behavior: "smooth" });
        }
    }, [messages]);

   const handleSendMessage = () => {
        if (query.trim() !== "") {
              setIsSending(true);
            const userMessageId = uuidv4();
              const newMessage = {
                id: userMessageId,
                sender: "user",
                text: query
              };
              setMessages(prevMessages => {
                 const updatedMessages = [...prevMessages, newMessage];
                  localStorage.setItem(localStorageKey, JSON.stringify(updatedMessages));
                  return updatedMessages;
              })
              currentMessageIdRef.current = uuidv4()
               currentMessageRef.current = ""
              const botMessage = {
                   id: currentMessageIdRef.current,
                  sender: 'bot',
                  text: ''
               };
               setMessages(prevMessages => {
                   const updatedMessages = [...prevMessages, botMessage];
                    localStorage.setItem(localStorageKey, JSON.stringify(updatedMessages));
                    return updatedMessages;
                })


              const wsURL = `ws://localhost:8080/chat?query=${encodeURIComponent(query)}`;
              if (socketRef.current && socketRef.current.readyState === WebSocket.OPEN) {
                   socketRef.current.send(JSON.stringify({ query }));
                  setQuery("");
                }
        }
    };


    const handleKeyDown = (event) => {
        if (event.key === "Enter") {
           handleSendMessage()
        }
    };

    const startListening = () => {
       setIsListening(true);
        recognitionRef.current = new window.webkitSpeechRecognition();
        recognitionRef.current.lang = 'en-US';

        recognitionRef.current.onresult = (event) => {
            const transcript = event.results[0][0].transcript;
            setQuery(transcript);
             setIsListening(false);
           handleSendMessage();
        };
        recognitionRef.current.onerror = (event) => {
           console.log("Speech recognition error", event)
             setIsListening(false);
        }
          recognitionRef.current.onend = () => {
           setIsListening(false);
        }
        recognitionRef.current.start();
    };

      const stopListening = () => {
       if(recognitionRef.current) {
         recognitionRef.current.stop();
         setIsListening(false);
       }
      }


   const handleVoiceButton = () => {
     if(isListening){
         stopListening();
    } else {
         startListening();
    }
    }

    return (
        <div className="chat-container">
            <div className="chat-header">
                <h1>Chat with Ollama</h1>
                <div className={`connection-status ${isConnected ? 'connected' : 'disconnected'}`}>
                    {isConnected ? 'Connected' : 'Disconnected'}
                </div>
            </div>

            <div className="message-area">
                {messages.map((message) => (
                    <div
                        key={message.id}
                        className={`message ${message.sender}`}
                       dangerouslySetInnerHTML={{__html: message.text}}
                     />
                 ))}
                <div ref={messagesEndRef} />
            </div>



            <div className="input-area">
                <input
                    type="text"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                   onKeyDown={handleKeyDown}
                    placeholder="Type your message..."
                   disabled={isSending || isListening}
                />
                <button disabled={isSending || isListening}  onClick={handleSendMessage}>
                    {isSending ? "Sending..." :  <FaPaperPlane />}
                </button>
                 <button disabled={isSending}  onClick={handleVoiceButton}>
                   {isListening ? "Listening..." : <FaMicrophone />}
                 </button>
            </div>
        </div>
    );
}

export default App;