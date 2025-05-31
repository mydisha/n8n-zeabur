FROM n8nio/n8n:latest

# Set environment variables
ENV NODE_ENV=production
ENV N8N_HOST=0.0.0.0
ENV N8N_PORT=5678
ENV N8N_PROTOCOL=https
ENV WEBHOOK_URL=https://your-app-name.zeabur.app/
ENV N8N_EDITOR_BASE_URL=https://your-app-name.zeabur.app/

# Create necessary directories
USER root
RUN mkdir -p /home/node/.n8n && chown -R node:node /home/node/.n8n

# Switch back to node user
USER node

# Expose port
EXPOSE 5678

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:5678/healthz || exit 1

# Start n8n
CMD ["n8n", "start"]
