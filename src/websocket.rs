use crate::{
    AppState, Sessions, generate_code,
    types::{Session, SignalingMessage},
};
use axum::{
    extract::{
        ConnectInfo, State, WebSocketUpgrade,
        ws::{Message, WebSocket},
    },
    response::IntoResponse,
};
use axum_extra::TypedHeader;
use std::{net::SocketAddr, ops::ControlFlow};
use tracing::{debug, error, info};

pub(crate) async fn handler(
    ws: WebSocketUpgrade,
    user_agent: Option<TypedHeader<headers::UserAgent>>,
    ConnectInfo(addr): ConnectInfo<SocketAddr>,
    State(state): State<AppState>,
) -> impl IntoResponse {
    let user_agent = if let Some(TypedHeader(user_agent)) = user_agent {
        user_agent.to_string()
    } else {
        String::from("unknown")
    };
    debug!("{user_agent} connected from {addr}");
    ws.on_upgrade(move |socket| handle_socket(socket, addr, state))
}

async fn handle_socket(mut socket: WebSocket, addr: SocketAddr, state: AppState) {
    if let Some(msg) = socket.recv().await {
        if let Ok(msg) = msg {
            if process_message(msg, addr, socket, state).await.is_break() {}
        } else {
            println!("client {addr} abruptly disconnected");
        }
    }
}

async fn process_message(
    msg: Message,
    addr: SocketAddr,
    mut socket: WebSocket,
    state: AppState,
) -> ControlFlow<(), ()> {
    debug!("{msg:?}");
    match msg {
        Message::Text(t) => {
            let message: SignalingMessage = match serde_json::from_str(t.as_str()) {
                Ok(msg) => msg,
                Err(err) => {
                    error!("{err}");
                    if let Err(err) = socket.send(Message::text("invalid message")).await {
                        error!("{err}");
                        return ControlFlow::Break(());
                    };
                    return ControlFlow::Continue(());
                }
            };
            match message {
                SignalingMessage::Offer {
                    passphrase_length,
                    sdp,
                } => {
                    if let Err(e) = process_offer(socket, passphrase_length, sdp, addr, state).await
                    {
                        error!("{e}");
                        return ControlFlow::Break(());
                    }
                }
                _ => (),
            }
        }
        Message::Close(c) => {
            if let Some(cf) = c {
                debug!(
                    "{addr} is disconnecting with code {} and reason \"{}\"",
                    cf.code, cf.reason
                );
            } else {
                debug!("{addr} is disconnecting")
            }

            return ControlFlow::Break(());
        }
        _ => (),
    }
    ControlFlow::Continue(())
}

async fn process_offer(
    mut socket: WebSocket,
    passphrase_length: u8,
    sdp: String,
    addr: SocketAddr,
    state: AppState,
) -> Result<(), Box<dyn std::error::Error>> {
    let code = generate_code(passphrase_length, state.wordlist);

    let message = SignalingMessage::Passphrase { passphrase: code };
    let json = serde_json::to_string(&message)?;
    socket.send(Message::Text(json.into())).await?;

    if let SignalingMessage::Passphrase { passphrase: code } = message {
        state.sessions.insert(
            code,
            Session {
                sender: addr.to_string(),
                offer: sdp,
                receiver: None,
            },
        );
    }

    Ok(())
}
