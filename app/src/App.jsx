import React, { useEffect, useRef, useState } from "react";
import { FaPaperPlane } from "react-icons/fa";
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

    useEffect(() => {
      const savedMessages = localStorage.getItem(localStorageKey);
        if (savedMessages) {
            setMessages(JSON.parse(savedMessages));
        }
         const connectWebSocket = () => {
             if (socketRef.current && socketRef.current.readyState === WebSocket.OPEN) {
                return;
             }

            const wsURL = `ws://localhost:8080/chat`;
            socketRef.current = new WebSocket(wsURL);

            socketRef.current.onopen = () => {
                setIsConnected(true);
                console.log("Connected to WebSocket");
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
                setIsConnected(false);
            };

            socketRef.current.onclose = () => {
                setIsSending(false);
                console.log("Disconnected from WebSocket");
                setIsConnected(false);
            };
        }
         connectWebSocket()
         return () => {
             if(socketRef.current){
                socketRef.current.close();
            }
        };
    }, []);

    useEffect(() => {
        if (messagesEndRef.current) {
            messagesEndRef.current.scrollIntoView({ behavior: "smooth" });
        }
    }, [messages]);

     const handleKeyDown = (event) => {
         if (event.key === "Enter" && query.trim() !== "") {
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

             });
            currentMessageIdRef.current = uuidv4();
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
                    disabled={isSending}
                />
                 <button disabled={isSending} >
                      {isSending ? "Sending..." :  <FaPaperPlane />}
                  </button>

             </div>
        </div>
    );
}

export default App;